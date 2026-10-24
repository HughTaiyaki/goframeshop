package repository_test

import (
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

// Shipping and receipt against real MySQL (task 0017): the conditional
// UPDATEs write and read back every shipping column, and the worker's Due
// scan picks up SHIPPED orders exactly when their confirm deadline passes.
func TestIntegrationShipConfirmAndAutoConfirmScan(t *testing.T) {
	ctx := integration(t)
	orders := repository.NewOrder()
	o := pendingOrder(t, ctx, 0)
	paid := time.Now()
	if ok, err := orders.Transition(ctx, o.ID, service.Change{
		From: []string{service.StatusPendingPayment}, To: service.StatusPaid, PaidAt: &paid,
	}); err != nil || !ok {
		t.Fatalf("pay: %v %v", ok, err)
	}

	shippedAt := time.Now().Truncate(time.Millisecond)
	deadline := shippedAt.Add(time.Hour)
	ship := service.Change{
		From: []string{service.StatusPaid}, To: service.StatusShipped,
		Shipment: &service.Shipment{Carrier: "顺丰", TrackingNo: "SF-IT-1"}, ShippedAt: &shippedAt, ConfirmDeadline: &deadline,
	}
	// Two back-office clicks at once: one ships, the other changes nothing.
	var wg sync.WaitGroup
	results := make([]bool, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := orders.Transition(ctx, o.ID, ship)
			if err != nil {
				t.Error(err)
			}
			results[i] = ok
		}()
	}
	wg.Wait()
	if results[0] == results[1] {
		t.Fatalf("both or neither shipped: %v", results)
	}
	got, err := orders.FindByNo(ctx, o.No)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != service.StatusShipped || got.Carrier != "顺丰" || got.TrackingNo != "SF-IT-1" ||
		!got.ShippedAt.Equal(shippedAt) || !got.ConfirmDeadline.Equal(deadline) {
		t.Fatalf("after shipping: %+v", got)
	}

	due := func(now time.Time) bool {
		all, err := orders.Due(ctx, now, 10000)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range all {
			if d.ID == o.ID {
				return true
			}
		}
		return false
	}
	if due(deadline.Add(-time.Second)) {
		t.Fatal("due before its confirm deadline")
	}
	if !due(deadline) {
		t.Fatal("not due at its confirm deadline")
	}

	completedAt := deadline
	complete := service.Change{
		From: []string{service.StatusShipped}, To: service.StatusCompleted,
		CompletedAt: &completedAt, CompleteReason: service.CompleteAutoConfirmed,
	}
	if ok, err := orders.Transition(ctx, o.ID, complete); err != nil || !ok {
		t.Fatalf("complete: %v %v", ok, err)
	}
	if ok, _ := orders.Transition(ctx, o.ID, complete); ok {
		t.Fatal("completed twice")
	}
	got, _ = orders.FindByNo(ctx, o.No)
	if got.Status != service.StatusCompleted || got.CompleteReason != service.CompleteAutoConfirmed || !got.CompletedAt.Equal(completedAt) {
		t.Fatalf("after completing: %+v", got)
	}
	if due(deadline.Add(time.Hour)) {
		t.Fatal("a completed order is still due")
	}
}
