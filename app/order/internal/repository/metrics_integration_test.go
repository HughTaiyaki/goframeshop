package repository_test

import (
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

// The backlog queries behind trade_workflow_backlog against real MySQL: an
// order stuck in RESERVING for a minute is counted under "reserve", and
// the oldest age reaches back at least to it.
func TestIntegrationWorkflowBacklog(t *testing.T) {
	ctx := integration(t)
	before, err := repository.Backlog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"reserve", "commit", "close", "tech_refund", "after_sale_refund", "outbox"} {
		if _, ok := before[step]; !ok {
			t.Fatalf("step %q missing from %v", step, before)
		}
	}

	o := pendingOrder(t, ctx, 0)
	stuckSince := time.Now().Add(-time.Minute).Truncate(time.Millisecond)
	if _, err := dao.OrderInfo.Ctx(ctx).Data(dao.OrderInfo.Columns().Status, service.StatusReserving,
		dao.OrderInfo.Columns().CreatedAt, stuckSince).Where(dao.OrderInfo.Columns().Id, o.ID).Update(); err != nil {
		t.Fatal(err)
	}
	after, err := repository.Backlog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reserve := after["reserve"]
	if reserve.Count != before["reserve"].Count+1 {
		t.Fatalf("reserve count %d -> %d, want one more", before["reserve"].Count, reserve.Count)
	}
	if reserve.Oldest.IsZero() || reserve.Oldest.After(stuckSince) {
		t.Fatalf("oldest reserve %v, want at or before %v", reserve.Oldest, stuckSince)
	}

	pendingBefore, err := repository.CountPendingPayment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pendingOrder(t, ctx, 1)
	if n, _ := repository.CountPendingPayment(ctx); n != pendingBefore+1 {
		t.Fatalf("pending payment %d -> %d, want one more", pendingBefore, n)
	}
}
