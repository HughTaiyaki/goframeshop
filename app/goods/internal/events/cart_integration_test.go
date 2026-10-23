package events_test

// The cart against the Compose MySQL (goods database): the one-statement
// upsert under concurrency, and the order.placed consumer through the real
// inbox.
//
//	GOFRAMESHOP_IT=1 go test ./app/goods/internal/events/ -v
//
// Every test uses its own random user id and deletes exactly its rows.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/events"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

func integration(t *testing.T) (context.Context, uint64) {
	t.Helper()
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL")
	}
	link := "mysql:shop:change-me@tcp(127.0.0.1:3306)/goods?loc=Local&parseTime=true"
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link, MaxOpenConnCount: 32}}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	userID := uint64(9_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	t.Cleanup(func() {
		db := dao.CartItem.DB()
		_, _ = db.Exec(ctx, "DELETE FROM cart_item WHERE user_id = ?", userID)
		_, _ = db.Exec(ctx, "DELETE FROM inbox_event WHERE consumer = ? AND event_id LIKE 'it-%'", events.CartConsumer)
	})
	return ctx, userID
}

func TestIntegrationConcurrentAddsMakeOneRow(t *testing.T) {
	ctx, userID := integration(t)
	repo := repository.NewCart()
	const goodsID = 1

	add := func(n int) (succeeded int64) {
		var wg sync.WaitGroup
		var ok atomic.Int64
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := repo.Add(ctx, userID, goodsID, 3, service.MaxCartQuantity)
				switch {
				case err == nil:
					ok.Add(1)
				case !errors.Is(err, service.ErrCartLimit):
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		return ok.Load()
	}

	// 20 × 3 fits: every add lands in the same row.
	if n := add(20); n != 20 {
		t.Fatalf("%d of 20 adds succeeded", n)
	}
	if count, _ := repo.Count(ctx, userID); count != 1 {
		t.Fatalf("%d rows, want 1", count)
	}
	row, _ := repo.FindByGoods(ctx, userID, goodsID)
	if row.Quantity != 60 {
		t.Fatalf("quantity %d, want 60", row.Quantity)
	}

	// 20 more would make 120: exactly 13 fit (60 + 39 = 99), the rest are
	// refused without changing the row.
	if n := add(20); n != 13 {
		t.Fatalf("%d adds succeeded past 60, want 13", n)
	}
	if row, _ = repo.FindByGoods(ctx, userID, goodsID); row.Quantity != 99 {
		t.Fatalf("quantity %d, want 99", row.Quantity)
	}
}

func placed(t *testing.T, p eventbus.OrderPlaced) amqp.Delivery {
	t.Helper()
	payload, _ := json.Marshal(p)
	body, _ := json.Marshal(eventbus.Envelope{SchemaVersion: 1, EventID: "it-" + uuid.NewString()[:30],
		Type: eventbus.TypeOrderPlaced, AggregateID: p.OrderNo, Payload: payload})
	return amqp.Delivery{Body: body}
}

func TestIntegrationOrderPlacedTakesGoodsOutOfTheCart(t *testing.T) {
	ctx, userID := integration(t)
	repo := repository.NewCart()
	quantities := func() map[uint64]uint32 {
		items, err := repo.List(ctx, userID, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		out := map[uint64]uint32{}
		for _, item := range items {
			out[item.GoodsID] = item.Quantity
		}
		return out
	}
	const kept, emptied, changedLater, other = 11, 12, 13, 14
	for goodsID, q := range map[uint64]uint32{kept: 3, emptied: 1, changedLater: 2, other: 5} {
		if _, err := repo.Add(ctx, userID, goodsID, q, service.MaxCartQuantity); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := repo.FindByGoods(ctx, userID, kept)

	// The order is placed now; afterwards the shopper changes one row.
	time.Sleep(5 * time.Millisecond)
	at := time.Now()
	time.Sleep(5 * time.Millisecond)
	row, _ := repo.FindByGoods(ctx, userID, changedLater)
	if ok, err := repo.SetQuantity(ctx, userID, row.ID, 4); err != nil || !ok {
		t.Fatalf("set quantity: %v %v", ok, err)
	}

	order := eventbus.OrderPlaced{
		OrderNo: "IT" + uuid.NewString()[:8], UserID: userID, PlacedAt: at, FromCart: true,
		Items: []eventbus.OrderPlacedItem{
			{GoodsID: kept, Quantity: 1},
			{GoodsID: emptied, Quantity: 1},
			{GoodsID: changedLater, Quantity: 2},
			{GoodsID: 99, Quantity: 1}, // never in the cart
		},
	}
	x := events.NewCart(dao.CartItem.DB())
	delivery := placed(t, order)
	if err := x.Handle(ctx, delivery); err != nil {
		t.Fatal(err)
	}
	want := map[uint64]uint32{kept: 2, changedLater: 4, other: 5}
	got := quantities()
	if len(got) != len(want) {
		t.Fatalf("cart %v, want %v", got, want)
	}
	for goodsID, q := range want {
		if got[goodsID] != q {
			t.Fatalf("cart %v, want %v", got, want)
		}
	}
	// The decrement keeps the shopper's own updated_at, so a second order
	// placed earlier than now still finds the row.
	after, _ := repo.FindByGoods(ctx, userID, kept)
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("updated_at moved from %v to %v", before.UpdatedAt, after.UpdatedAt)
	}

	// The same event again: the inbox makes it a no-op.
	if err := x.Handle(ctx, delivery); err != nil {
		t.Fatal(err)
	}
	if got := quantities(); got[kept] != 2 {
		t.Fatalf("redelivery applied again: %v", got)
	}

	// An order not from the cart leaves the cart alone.
	order.FromCart, order.OrderNo = false, "IT"+uuid.NewString()[:8]
	if err := x.Handle(ctx, placed(t, order)); err != nil {
		t.Fatal(err)
	}
	if got := quantities(); got[kept] != 2 || got[other] != 5 {
		t.Fatalf("buy-now order changed the cart: %v", got)
	}
}
