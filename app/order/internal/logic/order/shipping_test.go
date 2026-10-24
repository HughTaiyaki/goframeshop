package order

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

var sf = service.Shipment{Carrier: "顺丰", TrackingNo: "SF1234567890"}

// paidOrder places an order and pays for it.
func paidOrder(t *testing.T, f *fixture, requestID string) *service.Order {
	t.Helper()
	ctx := context.Background()
	o := placed(t, f, requestID)
	view, err := f.svc.Pay(ctx, alice, o.No)
	if err != nil {
		t.Fatal(err)
	}
	f.provider.pay(view.PaymentID)
	if err := f.notify(t, view.PaymentID); err != nil {
		t.Fatal(err)
	}
	if got := f.status(t, o.No); got.Status != service.StatusPaid {
		t.Fatalf("not paid: %s", got.Status)
	}
	return o
}

func TestShipThenConfirm(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := paidOrder(t, f, "r1")

	shipped, err := f.svc.Ship(ctx, o.No, service.Shipment{Carrier: " 顺丰 ", TrackingNo: "SF1234567890"})
	if err != nil {
		t.Fatal(err)
	}
	if shipped.Status != service.StatusShipped || shipped.Carrier != "顺丰" || shipped.TrackingNo != "SF1234567890" ||
		shipped.ShippedAt == nil || !shipped.ConfirmDeadline.Equal(shipped.ShippedAt.Add(timing.AutoConfirm)) {
		t.Fatalf("shipped: %+v", shipped)
	}

	// The same shipment again is a no-op; a different one is refused.
	if again, err := f.svc.Ship(ctx, o.No, sf); err != nil || !again.ShippedAt.Equal(*shipped.ShippedAt) {
		t.Fatalf("ship again: %+v %v", again, err)
	}
	if _, err := f.svc.Ship(ctx, o.No, service.Shipment{Carrier: "顺丰", TrackingNo: "SF999"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("other waybill: err = %v, want ErrFailedPrecondition", err)
	}

	// Only the buyer can confirm; to anyone else the order does not exist.
	if _, err := f.svc.ConfirmReceipt(ctx, bob, o.No); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("bob confirms: err = %v, want ErrNotFound", err)
	}
	f.advance(time.Hour)
	done, err := f.svc.ConfirmReceipt(ctx, alice, o.No)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != service.StatusCompleted || done.CompleteReason != service.CompleteUserConfirmed ||
		!done.CompletedAt.Equal(f.clock()) {
		t.Fatalf("confirmed: %+v", done)
	}
	if again, err := f.svc.ConfirmReceipt(ctx, alice, o.No); err != nil || again.Status != service.StatusCompleted {
		t.Fatalf("confirm again: %+v %v", again, err)
	}
	// Shipping a completed order with its own waybill still answers with it.
	if again, err := f.svc.Ship(ctx, o.No, sf); err != nil || again.Status != service.StatusCompleted {
		t.Fatalf("ship completed: %+v %v", again, err)
	}
}

func TestOnlyPaidOrdersShipAndOnlyShippedOnesComplete(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	unpaid := placed(t, f, "r1")
	if _, err := f.svc.Ship(ctx, unpaid.No, sf); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("ship unpaid: err = %v", err)
	}
	if _, err := f.svc.ConfirmReceipt(ctx, alice, unpaid.No); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("confirm unpaid: err = %v", err)
	}
	paid := paidOrder(t, f, "r2")
	if _, err := f.svc.ConfirmReceipt(ctx, alice, paid.No); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("confirm before shipping: err = %v", err)
	}
	if _, err := f.svc.Ship(ctx, "nope", sf); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("ship unknown: err = %v", err)
	}
}

func TestShipmentIsValidated(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := paidOrder(t, f, "r1")
	long := make([]byte, service.MaxTrackingNoBytes+1)
	for i := range long {
		long[i] = '1'
	}
	for name, in := range map[string]service.Shipment{
		"no carrier":         {Carrier: " ", TrackingNo: "SF1"},
		"carrier too long":   {Carrier: string(make([]rune, service.MaxCarrierRunes+1)), TrackingNo: "SF1"},
		"no waybill":         {Carrier: "顺丰"},
		"waybill too long":   {Carrier: "顺丰", TrackingNo: string(long)},
		"waybill with space": {Carrier: "顺丰", TrackingNo: "SF 1"},
		"waybill with tag":   {Carrier: "顺丰", TrackingNo: "<b>1</b>"},
	} {
		if _, err := f.svc.Ship(ctx, o.No, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Fatalf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
	if got := f.status(t, o.No); got.Status != service.StatusPaid {
		t.Fatalf("a refused shipment changed the order: %s", got.Status)
	}
}

func TestUnconfirmedOrdersCompleteOnTheirOwn(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	early := paidOrder(t, f, "r1")
	if _, err := f.svc.Ship(ctx, early.No, sf); err != nil {
		t.Fatal(err)
	}
	f.advance(time.Hour)
	late := paidOrder(t, f, "r2")
	if _, err := f.svc.Ship(ctx, late.No, service.Shipment{Carrier: "顺丰", TrackingNo: "SF2"}); err != nil {
		t.Fatal(err)
	}

	// Just before the first deadline nothing happens.
	f.advance(timing.AutoConfirm - time.Hour - time.Second)
	if err := f.svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.status(t, early.No); got.Status != service.StatusShipped {
		t.Fatalf("completed before its deadline: %s", got.Status)
	}
	// Past it, the first completes and the second still waits.
	f.advance(time.Second)
	if err := f.svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.status(t, early.No); got.Status != service.StatusCompleted || got.CompleteReason != service.CompleteAutoConfirmed {
		t.Fatalf("early: %s %s", got.Status, got.CompleteReason)
	}
	if got := f.status(t, late.No); got.Status != service.StatusShipped {
		t.Fatalf("late completed early: %s", got.Status)
	}
	// The shopper confirming afterwards just gets the completed order.
	if got, err := f.svc.ConfirmReceipt(ctx, alice, early.No); err != nil || got.CompleteReason != service.CompleteAutoConfirmed {
		t.Fatalf("confirm after auto: %+v %v", got, err)
	}
}

func TestConfirmAndAutoConfirmRace(t *testing.T) {
	ctx := context.Background()
	for run := 0; run < 50; run++ {
		f := newFixture()
		o := paidOrder(t, f, "r1")
		if _, err := f.svc.Ship(ctx, o.No, sf); err != nil {
			t.Fatal(err)
		}
		f.advance(timing.AutoConfirm)
		var wg sync.WaitGroup
		var confirmErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, confirmErr = f.svc.ConfirmReceipt(ctx, alice, o.No) }()
		go func() { defer wg.Done(); _ = f.svc.Recover(ctx) }()
		wg.Wait()
		got := f.status(t, o.No)
		if confirmErr != nil || got.Status != service.StatusCompleted || got.CompleteReason == "" {
			t.Fatalf("run %d: %s %q %v", run, got.Status, got.CompleteReason, confirmErr)
		}
	}
}

func TestAfterSaleRequestsHoldShipping(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := paidOrder(t, f, "r1") // goods 1 and 2
	got := f.status(t, o.No)
	request := &service.Refund{ID: 1, OrderID: got.ID, GoodsID: 1, Status: service.RefundPending}
	f.refunds.rows = []*service.Refund{request}

	// Waiting for review: decide first, then ship.
	if _, err := f.svc.Ship(ctx, o.No, sf); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("ship with a pending request: err = %v", err)
	}
	// Rejected: ship everything.
	request.Status = service.RefundRejected
	if shipped, err := f.svc.Ship(ctx, o.No, sf); err != nil || shipped.Status != service.StatusShipped {
		t.Fatalf("ship after a rejection: %+v %v", shipped, err)
	}

	// One line refunded: the other still ships.
	partial := paidOrder(t, f, "r2")
	pid := f.status(t, partial.No).ID
	f.refunds.rows = []*service.Refund{{ID: 2, OrderID: pid, GoodsID: 1, Status: service.RefundRefunded}}
	if _, err := f.svc.Ship(ctx, partial.No, service.Shipment{Carrier: "顺丰", TrackingNo: "SF2"}); err != nil {
		t.Fatalf("ship the remaining line: %v", err)
	}

	// Every line approved or refunded: nothing left to send.
	all := paidOrder(t, f, "r3")
	aid := f.status(t, all.No).ID
	f.refunds.rows = []*service.Refund{
		{ID: 3, OrderID: aid, GoodsID: 1, Status: service.RefundRefunded},
		{ID: 4, OrderID: aid, GoodsID: 2, Status: service.RefundApproved},
	}
	if _, err := f.svc.Ship(ctx, all.No, service.Shipment{Carrier: "顺丰", TrackingNo: "SF3"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("ship a fully refunded order: err = %v", err)
	}
	if got := f.status(t, all.No); got.Status != service.StatusPaid {
		t.Fatalf("status %s", got.Status)
	}
}
