package order

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry/logcapture"
)

// transitions lists "from->to" of the order.transition events for orderNo.
func transitions(logs *logcapture.Recorder, orderNo string) []string {
	var out []string
	for _, e := range logs.Events(telemetry.EventOrderTransition) {
		if e["order_id"] == orderNo {
			out = append(out, fmt.Sprintf("%v->%v", e.Attr("from"), e.Attr("to")))
		}
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A paid order logs every state it went through, once, and the payment it
// observed (design §9.1).
func TestPaidOrderLogsItsTransitionsAndPayment(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID)
	if err := f.notify(t, view.PaymentID); err != nil {
		t.Fatal(err)
	}
	// The provider repeats its notification: no second transition.
	if err := f.notify(t, view.PaymentID); err != nil {
		t.Fatal(err)
	}

	want := []string{"RESERVING->PENDING_PAYMENT", "PENDING_PAYMENT->PAYMENT_CONFIRMING", "PAYMENT_CONFIRMING->PAID"}
	if got := transitions(logs, o.No); !equal(got, want) {
		t.Fatalf("transitions %v, want %v", got, want)
	}
	observed := logs.Events(telemetry.EventPaymentObserved)
	if len(observed) != 2 || observed[0]["order_id"] != o.No || observed[0].Attr("payment_id") != view.PaymentID ||
		observed[0].Attr("outcome") != service.PaidConfirming || observed[1].Attr("outcome") != service.PaidDuplicate {
		t.Fatalf("payment.observed: %v", observed)
	}
}

func TestCancelledOrderLogsWhyItClosed(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	if _, err := f.svc.Cancel(ctx, alice, o.No); err != nil {
		t.Fatal(err)
	}
	var closing logcapture.Line
	for _, e := range logs.Events(telemetry.EventOrderTransition) {
		if e.Attr("to") == service.StatusClosing {
			closing = e
		}
	}
	if closing == nil || closing.Attr("from") != service.StatusPendingPayment || closing.Attr("reason") != service.ReasonUserCancelled {
		t.Fatalf("closing transition: %v", closing)
	}
}

// A refused commit: EXCEPTION, a technical refund, then CLOSING once it is
// back, each logged.
func TestTechnicalRefundIsLogged(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID)
	f.inventory.commits = []reply{{outcome: service.OutcomeStateConflict}}
	f.notify(t, view.PaymentID)
	f.svc.Recover(ctx)

	got := transitions(logs, o.No)
	want := []string{"PAYMENT_CONFIRMING->EXCEPTION", "EXCEPTION->CLOSING"}
	for _, w := range want {
		found := false
		for _, g := range got {
			found = found || g == w
		}
		if !found {
			t.Fatalf("transition %s missing from %v", w, got)
		}
	}
	refunds := logs.Events(telemetry.EventPaymentRefundRequest)
	if len(refunds) != 1 || refunds[0]["order_id"] != o.No || refunds[0].Attr("kind") != "technical" ||
		refunds[0].Attr("refund_id") != service.TechRefundID(view.PaymentID) || refunds[0].Attr("outcome") != "succeeded" ||
		refunds[0]["error.type"] != "" {
		t.Fatalf("payment.refund.requested: %v", refunds)
	}
}

// The worker logs what it retries; a step it merely continues (an expired
// order closing) is not a retry.
func TestRecoveryLogsRetries(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	f := newFixture()
	f.inventory.reserves = []reply{{err: errUnavailable}}
	o, _ := f.svc.Create(ctx, alice, input("r1"))
	f.svc.Recover(ctx)

	retries := logs.Events(telemetry.EventWorkflowRetry)
	if len(retries) != 1 || retries[0]["order_id"] != o.No || retries[0].Attr("step") != "reserve" {
		t.Fatalf("workflow.retry: %v", retries)
	}
	f.advance(timing.PaymentTimeout + 1)
	f.svc.Recover(ctx) // expired PENDING_PAYMENT: closes, not a retry
	if n := len(logs.Events(telemetry.EventWorkflowRetry)); n != 1 {
		t.Fatalf("%d retries after an expiry, want still 1", n)
	}
}

// trade_order_create_total's outcome label, for every way a submit ends.
func TestCreateOutcome(t *testing.T) {
	order := func(status string) *service.Order { return &service.Order{Status: status} }
	for name, c := range map[string]struct {
		order     *service.Order
		err       error
		duplicate bool
		want      string
	}{
		"payable":           {order(service.StatusPendingPayment), nil, false, "pending_payment"},
		"reservation open":  {order(service.StatusReserving), nil, false, "reserving"},
		"no stock":          {order(service.StatusClosed), nil, false, "rejected"},
		"retry of a submit": {order(service.StatusPendingPayment), nil, true, "duplicate"},
		"bad request":       {nil, service.ErrInvalidArgument, false, "invalid"},
		"request id reused": {nil, service.ErrIdempotencyConflict, false, "invalid"},
		"goods off shelf":   {nil, service.ErrGoodsUnavailable, false, "rejected"},
		"database down":     {nil, errors.New("connection refused"), false, "error"},
	} {
		if got := createOutcome(c.order, c.err, c.duplicate); got != c.want {
			t.Errorf("%s: %s, want %s", name, got, c.want)
		}
	}
}
