package repository_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

// After-sale review against real MySQL (task 0018): the conditional UPDATE
// decides an approve racing a reject once, the review columns round-trip,
// and the worker's scan and the shipping guard's per-order read see them.
func TestIntegrationRefundReview(t *testing.T) {
	ctx := integration(t)
	orders := repository.NewOrder()
	refunds := repository.NewRefund(orders)
	o := pendingOrder(t, ctx, 0)
	no := fmt.Sprintf("RIT%d", time.Now().UnixNano()%1e15)
	id, err := refunds.Create(ctx, &service.Refund{
		No: no, OrderID: o.ID, UserID: o.UserID, GoodsID: 1, Reason: "it", Status: service.RefundPending, AmountCents: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dao.RefundInfo.Ctx(ctx).Where(dao.RefundInfo.Columns().Id, id).Delete() })

	now := time.Now().Truncate(time.Millisecond)
	approve, reject := "ok", "no"
	var wg sync.WaitGroup
	won := make([]bool, 2)
	for i, c := range []service.RefundChange{
		{From: service.RefundPending, To: service.RefundApproved, ReviewNote: &approve, ReviewedAt: &now},
		{From: service.RefundPending, To: service.RefundRejected, ReviewNote: &reject, ReviewedAt: &now},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			changed, err := refunds.Transition(ctx, id, c)
			if err != nil {
				t.Error(err)
			}
			won[i] = changed
		}()
	}
	wg.Wait()
	if won[0] == won[1] {
		t.Fatalf("approve and reject both or neither won: %v", won)
	}
	got, err := refunds.FindByNo(ctx, no)
	if err != nil {
		t.Fatal(err)
	}
	if got.AmountCents != 100 || got.GoodsName != "it" || got.OrderNo != o.No || !got.ReviewedAt.Equal(now) {
		t.Fatalf("read back: %+v", got)
	}
	if (won[0] && (got.Status != service.RefundApproved || got.ReviewNote != "ok")) ||
		(won[1] && (got.Status != service.RefundRejected || got.ReviewNote != "no")) {
		t.Fatalf("winner %v, row %+v", won, got)
	}

	byOrder, err := refunds.ByOrder(ctx, o.ID)
	if err != nil || len(byOrder) != 1 || byOrder[0].No != no {
		t.Fatalf("by order: %+v %v", byOrder, err)
	}
	inDue := func() bool {
		due, err := refunds.DueApproved(ctx, 10000)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range due {
			if r.ID == id {
				return true
			}
		}
		return false
	}
	if inDue() != won[0] {
		t.Fatalf("due = %v, approved = %v", inDue(), won[0])
	}
	if won[0] {
		if err := refunds.Attempted(ctx, id); err != nil {
			t.Fatal(err)
		}
		if ok, err := refunds.Transition(ctx, id, service.RefundChange{
			From: service.RefundApproved, To: service.RefundRefunded, RefundedAt: &now,
		}); err != nil || !ok {
			t.Fatalf("refunded: %v %v", ok, err)
		}
		got, _ = refunds.FindByNo(ctx, no)
		if got.Status != service.RefundRefunded || got.Attempts != 1 || !got.RefundedAt.Equal(now) || inDue() {
			t.Fatalf("after refunding: %+v", got)
		}
	}
}
