package repository_test

// Integration tests for the reservation ledger against a real MySQL. The
// in-memory fake serialises everything behind one mutex, so it cannot show
// that row locks, READ COMMITTED and the deadlock retry actually hold up when
// transactions really overlap. These tests can.
//
// They are opt-in, because they need the Compose MySQL with the goods schema:
//
//	make infra-up && make db-init
//	GOFRAMESHOP_IT=1 go test ./app/goods/internal/repository/ -run Integration -v
//
// GOODS_IT_LINK overrides the connection. Every test creates its own goods
// and order ids and deletes exactly those rows afterwards.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic/inventory"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/ittest"
)

func integration(t *testing.T) context.Context {
	t.Helper()
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL")
	}
	link := os.Getenv("GOODS_IT_LINK")
	if link == "" {
		link = ittest.MySQL(t, "goods")
	}
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link, MaxOpenConnCount: 64}}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := dao.Inventory.DB().Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("cannot reach MySQL: %v", err)
	}
	return ctx
}

// newGoods creates an on-shelf product with stock and removes everything the
// test did to it afterwards.
func newGoods(t *testing.T, ctx context.Context, available uint32) uint64 {
	t.Helper()
	// Any existing level-1 category id is fine; the ledger never looks at it.
	id, err := repository.NewGoods().Create(ctx, &entity.GoodsInfo{
		Name: "IT " + t.Name(), PriceCents: 1, Level1CategoryId: 1, Status: service.StatusOnShelf,
	}, available)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db := dao.Inventory.DB()
		_, _ = db.Exec(ctx, "DELETE FROM inventory_operation WHERE order_id IN (SELECT order_id FROM reservation WHERE goods_id = ?)", id)
		_, _ = db.Exec(ctx, "DELETE FROM reservation WHERE goods_id = ?", id)
		_, _ = db.Exec(ctx, "DELETE FROM inventory WHERE goods_id = ?", id)
		_, _ = db.Exec(ctx, "DELETE FROM goods_info WHERE id = ?", id)
	})
	return id
}

// cleanupOperations removes operation rows that have no reservation row
// (refusals), which the goods cleanup above cannot find by goods id.
func cleanupOperations(t *testing.T, ctx context.Context, prefix string) {
	t.Cleanup(func() {
		_, _ = dao.Inventory.DB().Exec(ctx, "DELETE FROM inventory_operation WHERE operation_id LIKE ?", prefix+"%")
	})
}

func stockOf(t *testing.T, ctx context.Context, goodsID uint64) service.Stock {
	t.Helper()
	stock, err := repository.NewStock().Find(ctx, goodsID)
	if err != nil {
		t.Fatal(err)
	}
	return stock
}

func TestIntegrationConcurrentReservesNeverOversell(t *testing.T) {
	ctx := integration(t)
	svc := inventory.New(repository.NewLedger())
	goodsID := newGoods(t, ctx, 10)
	prefix := fmt.Sprintf("it-oversell-%d-", time.Now().UnixNano())
	cleanupOperations(t, ctx, prefix)

	const buyers = 40
	var wg sync.WaitGroup
	outcomes := make(chan string, buyers)
	errs := make(chan error, buyers)
	for i := range buyers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			order := fmt.Sprintf("%s%d", prefix, i)
			r, err := svc.Reserve(ctx, order, order, []service.InventoryItem{{GoodsID: goodsID, Quantity: 1}})
			if err != nil {
				errs <- err
				return
			}
			outcomes <- r.Outcome
		}(i)
	}
	wg.Wait()
	close(outcomes)
	close(errs)
	for err := range errs {
		t.Fatalf("reserve failed: %v", err)
	}
	counts := map[string]int{}
	for o := range outcomes {
		counts[o]++
	}
	if counts[service.OutcomeSucceeded] != 10 || counts[service.OutcomeInsufficientStock] != buyers-10 {
		t.Fatalf("outcomes = %v, want 10 succeeded and %d insufficient", counts, buyers-10)
	}
	if got := stockOf(t, ctx, goodsID); got != (service.Stock{Available: 0, Reserved: 10}) {
		t.Fatalf("stock = %+v, want available 0, reserved 10", got)
	}
}

func TestIntegrationSameOperationIDActsOnce(t *testing.T) {
	ctx := integration(t)
	svc := inventory.New(repository.NewLedger())
	goodsID := newGoods(t, ctx, 100)
	op := fmt.Sprintf("it-once-%d", time.Now().UnixNano())
	cleanupOperations(t, ctx, op)

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := svc.Reserve(ctx, op, op, []service.InventoryItem{{GoodsID: goodsID, Quantity: 3}})
			if err == nil && r.Outcome != service.OutcomeSucceeded {
				err = fmt.Errorf("outcome %s", r.Outcome)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a retry of the same operation failed: %v", err)
		}
	}
	if got := stockOf(t, ctx, goodsID); got != (service.Stock{Available: 97, Reserved: 3}) {
		t.Fatalf("stock = %+v: 20 concurrent copies of one operation must reserve once", got)
	}
}

// Many orders reserving several goods in different item orders: the
// goods_id lock order must keep them from deadlocking (or the retry must
// absorb it), and the totals must add up.
func TestIntegrationMultiItemOrdersStayConsistent(t *testing.T) {
	ctx := integration(t)
	svc := inventory.New(repository.NewLedger())
	a, b := newGoods(t, ctx, 1000), newGoods(t, ctx, 1000)
	prefix := fmt.Sprintf("it-multi-%d-", time.Now().UnixNano())
	cleanupOperations(t, ctx, prefix)

	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := range 30 {
		wg.Add(2)
		order := fmt.Sprintf("%s%d", prefix, i)
		items := []service.InventoryItem{{GoodsID: a, Quantity: 2}, {GoodsID: b, Quantity: 1}}
		if i%2 == 1 {
			items[0], items[1] = items[1], items[0] // callers list goods in any order
		}
		go func() {
			defer wg.Done()
			_, err := svc.Reserve(ctx, "r-"+order, order, items)
			errs <- err
		}()
		// Half of the orders are released concurrently: some Releases win
		// the race and leave tombstones, some release a live reservation.
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				_, err := svc.Release(ctx, "x-"+order, order, items)
				errs <- err
				return
			}
			errs <- nil
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("operation failed: %v", err)
		}
	}
	reserved, err := dao.Reservation.Ctx(ctx).
		Where(dao.Reservation.Columns().GoodsId, a).
		Where(dao.Reservation.Columns().State, service.StateReserved).
		Sum(dao.Reservation.Columns().Quantity)
	if err != nil {
		t.Fatal(err)
	}
	stockA, stockB := stockOf(t, ctx, a), stockOf(t, ctx, b)
	// Conservation: nothing created or lost, and the ledger agrees with the
	// reservation rows.
	if stockA.Available+stockA.Reserved != 1000 || stockB.Available+stockB.Reserved != 1000 {
		t.Fatalf("stock not conserved: a=%+v b=%+v", stockA, stockB)
	}
	if uint32(reserved) != stockA.Reserved || stockA.Reserved != 2*stockB.Reserved {
		t.Fatalf("ledger disagrees with reservations: rows=%v a=%+v b=%+v", reserved, stockA, stockB)
	}
}
