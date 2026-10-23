package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/testfake"
)

func setup() (service.IInventory, *testfake.Ledger) {
	ledger := testfake.NewLedger()
	ledger.AddGoods(1, 10)
	ledger.AddGoods(2, 3)
	return New(ledger), ledger
}

func items(pairs ...uint64) []service.InventoryItem {
	out := []service.InventoryItem{}
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, service.InventoryItem{GoodsID: pairs[i], Quantity: uint32(pairs[i+1])})
	}
	return out
}

func expectStock(t *testing.T, l *testfake.Ledger, goodsID uint64, available, reserved, sold uint32) {
	t.Helper()
	got := l.Stocks[goodsID]
	if got != (service.Stock{Available: available, Reserved: reserved, Sold: sold}) {
		t.Fatalf("goods %d stock = %+v, want available=%d reserved=%d sold=%d", goodsID, got, available, reserved, sold)
	}
}

func expectOutcome(t *testing.T, result *service.OperationResult, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if result.Outcome != want {
		t.Fatalf("outcome = %s, want %s", result.Outcome, want)
	}
}

func TestReserveCommitMovesStockThroughTheLedger(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()

	r, err := svc.Reserve(ctx, "reserve-o1", "o1", items(2, 1, 1, 4))
	expectOutcome(t, r, err, service.OutcomeSucceeded)
	if len(r.Reservations) != 2 || r.Reservations[0].GoodsID != 1 || r.Reservations[0].State != service.StateReserved {
		t.Fatalf("reservations = %+v", r.Reservations)
	}
	expectStock(t, l, 1, 6, 4, 0)
	expectStock(t, l, 2, 2, 1, 0)

	c, err := svc.Commit(ctx, "commit-o1", "o1")
	expectOutcome(t, c, err, service.OutcomeSucceeded)
	expectStock(t, l, 1, 6, 0, 4)
	expectStock(t, l, 2, 2, 0, 1)
	for _, res := range c.Reservations {
		if res.State != service.StateCommitted {
			t.Fatalf("after commit: %+v", c.Reservations)
		}
	}
}

func TestReserveIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()

	// Goods 1 has enough, goods 2 does not: nothing may change.
	r, err := svc.Reserve(ctx, "op", "o1", items(1, 5, 2, 4))
	expectOutcome(t, r, err, service.OutcomeInsufficientStock)
	expectStock(t, l, 1, 10, 0, 0)
	expectStock(t, l, 2, 3, 0, 0)
	if len(r.Reservations) != 0 {
		t.Fatalf("a refused reserve left reservations: %+v", r.Reservations)
	}

	l.Sellable[2] = false // off shelf
	r, err = svc.Reserve(ctx, "op-2", "o2", items(1, 1, 2, 1))
	expectOutcome(t, r, err, service.OutcomeGoodsUnavailable)
	r, err = svc.Reserve(ctx, "op-3", "o3", items(99, 1))
	expectOutcome(t, r, err, service.OutcomeGoodsUnavailable)
	expectStock(t, l, 1, 10, 0, 0)
}

func TestRetriesReturnTheOriginalOutcomeWithoutActingAgain(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()

	first, err := svc.Reserve(ctx, "op", "o1", items(1, 2))
	expectOutcome(t, first, err, service.OutcomeSucceeded)
	// Same id, same request (item order and splitting do not matter).
	again, err := svc.Reserve(ctx, "op", "o1", items(1, 1, 1, 1))
	expectOutcome(t, again, err, service.OutcomeSucceeded)
	expectStock(t, l, 1, 8, 2, 0)

	// A refusal is stored too: stock arriving later does not change the
	// answer for that operation id.
	refused, _ := svc.Reserve(ctx, "op-big", "o2", items(2, 5))
	expectOutcome(t, refused, nil, service.OutcomeInsufficientStock)
	l.Stocks[2] = service.Stock{Available: 100}
	refusedAgain, err := svc.Reserve(ctx, "op-big", "o2", items(2, 5))
	expectOutcome(t, refusedAgain, err, service.OutcomeInsufficientStock)
}

func TestReusedOperationIDWithDifferentParametersIsRefused(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()
	if _, err := svc.Reserve(ctx, "op", "o1", items(1, 2)); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"other quantity": func() error { _, err := svc.Reserve(ctx, "op", "o1", items(1, 3)); return err },
		"other order":    func() error { _, err := svc.Reserve(ctx, "op", "o2", items(1, 2)); return err },
		"other kind":     func() error { _, err := svc.Commit(ctx, "op", "o1"); return err },
	} {
		if err := call(); !errors.Is(err, service.ErrIdempotencyConflict) {
			t.Errorf("%s: err = %v, want ErrIdempotencyConflict", name, err)
		}
	}
	expectStock(t, l, 1, 8, 2, 0)
}

// §6.3 row "none": Commit before Reserve is NOT_READY and not stored, so the
// same operation id still works once the reservation exists.
func TestCommitBeforeReserveIsNotReadyAndRetryable(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()

	c, err := svc.Commit(ctx, "commit-o1", "o1")
	expectOutcome(t, c, err, service.OutcomeNotReady)
	if _, stored := l.Operations["commit-o1"]; stored {
		t.Fatal("NOT_READY was stored; the retry would be stuck with it")
	}
	if _, err := svc.Reserve(ctx, "reserve-o1", "o1", items(1, 1)); err != nil {
		t.Fatal(err)
	}
	c, err = svc.Commit(ctx, "commit-o1", "o1")
	expectOutcome(t, c, err, service.OutcomeSucceeded)
	expectStock(t, l, 1, 9, 0, 1)
}

// §6.3 row "none": Release before Reserve writes a tombstone; the late
// Reserve is refused and takes no stock.
func TestReleaseBeforeReserveBlocksTheLateReserve(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()

	rel, err := svc.Release(ctx, "release-o1", "o1", items(1, 2))
	expectOutcome(t, rel, err, service.OutcomeSucceeded)
	if len(rel.Reservations) != 1 || rel.Reservations[0].State != service.StateReleased {
		t.Fatalf("no tombstone: %+v", rel.Reservations)
	}
	expectStock(t, l, 1, 10, 0, 0) // a tombstone gives nothing back

	late, err := svc.Reserve(ctx, "reserve-o1", "o1", items(1, 2))
	expectOutcome(t, late, err, service.OutcomeAlreadyReleased)
	expectStock(t, l, 1, 10, 0, 0)
}

// §6.3 rows RESERVED / COMMITTED / RELEASED for Commit and Release.
func TestStateTransitions(t *testing.T) {
	ctx := context.Background()

	t.Run("release a reservation returns it once", func(t *testing.T) {
		svc, l := setup()
		_, _ = svc.Reserve(ctx, "r", "o", items(1, 3))
		rel, err := svc.Release(ctx, "rel", "o", items(1, 3))
		expectOutcome(t, rel, err, service.OutcomeSucceeded)
		expectStock(t, l, 1, 10, 0, 0)
		// A second Release (new id) of a RELEASED row changes nothing.
		rel2, err := svc.Release(ctx, "rel-2", "o", items(1, 3))
		expectOutcome(t, rel2, err, service.OutcomeSucceeded)
		expectStock(t, l, 1, 10, 0, 0)
	})

	t.Run("sold stock is never released", func(t *testing.T) {
		svc, l := setup()
		_, _ = svc.Reserve(ctx, "r", "o", items(1, 3))
		_, _ = svc.Commit(ctx, "c", "o")
		rel, err := svc.Release(ctx, "rel", "o", items(1, 3))
		expectOutcome(t, rel, err, service.OutcomeStateConflict)
		expectStock(t, l, 1, 7, 0, 3)
	})

	t.Run("commit of a released reservation conflicts", func(t *testing.T) {
		svc, l := setup()
		_, _ = svc.Reserve(ctx, "r", "o", items(1, 3))
		_, _ = svc.Release(ctx, "rel", "o", items(1, 3))
		c, err := svc.Commit(ctx, "c", "o")
		expectOutcome(t, c, err, service.OutcomeStateConflict)
		expectStock(t, l, 1, 10, 0, 0)
	})

	t.Run("commit twice with new ids is harmless", func(t *testing.T) {
		svc, l := setup()
		_, _ = svc.Reserve(ctx, "r", "o", items(1, 3))
		_, _ = svc.Commit(ctx, "c", "o")
		c, err := svc.Commit(ctx, "c-2", "o")
		expectOutcome(t, c, err, service.OutcomeSucceeded)
		expectStock(t, l, 1, 7, 0, 3)
	})

	t.Run("reserve again for the same order changes nothing", func(t *testing.T) {
		svc, l := setup()
		_, _ = svc.Reserve(ctx, "r", "o", items(1, 3))
		same, err := svc.Reserve(ctx, "r-2", "o", items(1, 3))
		expectOutcome(t, same, err, service.OutcomeSucceeded)
		expectStock(t, l, 1, 7, 3, 0)
		different, err := svc.Reserve(ctx, "r-3", "o", items(1, 4))
		expectOutcome(t, different, err, service.OutcomeStateConflict)
		expectStock(t, l, 1, 7, 3, 0)
	})

	t.Run("partial release with one committed item changes nothing", func(t *testing.T) {
		svc, l := setup()
		_, _ = svc.Reserve(ctx, "r", "o", items(1, 1, 2, 1))
		_, _ = svc.Commit(ctx, "c", "o")
		rel, err := svc.Release(ctx, "rel", "o", items(1, 1, 2, 1))
		expectOutcome(t, rel, err, service.OutcomeStateConflict)
		expectStock(t, l, 1, 9, 0, 1)
		expectStock(t, l, 2, 2, 0, 1)
	})
}

// A concurrent call with the same operation id commits first: we roll back
// and return its result instead of acting a second time.
func TestLosingTheOperationIDRaceReturnsTheWinner(t *testing.T) {
	ctx := context.Background()
	svc, l := setup()
	l.RaceOn = "op"
	l.RaceOutcome = service.OutcomeInsufficientStock

	r, err := svc.Reserve(ctx, "op", "o1", items(1, 2))
	expectOutcome(t, r, err, service.OutcomeInsufficientStock)
	expectStock(t, l, 1, 10, 0, 0)
}

func TestQuery(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup()
	if r, err := svc.Query(ctx, "unknown"); r != nil || err != nil {
		t.Fatalf("unknown id: %+v %v", r, err)
	}
	_, _ = svc.Reserve(ctx, "op", "o1", items(1, 2))
	_, _ = svc.Release(ctx, "rel", "o1", items(1, 2))
	// The original outcome stays SUCCEEDED; the current state says released.
	r, err := svc.Query(ctx, "op")
	expectOutcome(t, r, err, service.OutcomeSucceeded)
	if r.Reservations[0].State != service.StateReleased {
		t.Fatalf("current state = %+v", r.Reservations)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup()
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'x'
	}
	tooMany := make([]service.InventoryItem, 51)
	for i := range tooMany {
		tooMany[i] = service.InventoryItem{GoodsID: uint64(i + 1), Quantity: 1}
	}
	bad := map[string]func() error{
		"empty op id":      func() error { _, err := svc.Reserve(ctx, "", "o", items(1, 1)); return err },
		"long order id":    func() error { _, err := svc.Reserve(ctx, "op", string(long), items(1, 1)); return err },
		"padded id":        func() error { _, err := svc.Reserve(ctx, " op", "o", items(1, 1)); return err },
		"no items":         func() error { _, err := svc.Reserve(ctx, "op", "o", []service.InventoryItem{}); return err },
		"zero quantity":    func() error { _, err := svc.Reserve(ctx, "op", "o", items(1, 0)); return err },
		"zero goods id":    func() error { _, err := svc.Reserve(ctx, "op", "o", items(0, 1)); return err },
		"merged too large": func() error { _, err := svc.Reserve(ctx, "op", "o", items(1, 6000, 1, 6000)); return err },
		"too many items":   func() error { _, err := svc.Reserve(ctx, "op", "o", tooMany); return err },
		"release no items": func() error { _, err := svc.Release(ctx, "op", "o", nil); return err },
	}
	for name, call := range bad {
		if err := call(); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
}
