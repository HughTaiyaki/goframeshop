package order

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var errUnavailable = errors.New("unavailable")

// placed creates a PENDING_PAYMENT order for alice.
func placed(t *testing.T, f *fixture, requestID string) *service.Order {
	t.Helper()
	o, err := f.svc.Create(context.Background(), alice, input(requestID))
	if err != nil || o.Status != service.StatusPendingPayment {
		t.Fatalf("create: %+v %v", o, err)
	}
	return o
}

// notice is what the simulator posts, signed "good" for fakeNotices.
func (f *fixture) notify(t *testing.T, paymentID string) error {
	t.Helper()
	body, _ := json.Marshal(utility.PaymentNotice{PaymentID: paymentID, State: service.PaymentSucceeded, NotifyTime: f.clock()})
	return f.svc.Notify(context.Background(), body, "good")
}

func (f *fixture) status(t *testing.T, no string) *service.Order {
	t.Helper()
	o, err := f.orders.FindByNo(context.Background(), no)
	if err != nil || o == nil {
		t.Fatalf("find %s: %v", no, err)
	}
	return o
}

func TestPayNotifyCommitPaid(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")

	view, err := f.svc.Pay(ctx, alice, o.No)
	if err != nil {
		t.Fatal(err)
	}
	if view.PaymentID != "P"+o.No || view.State != service.PaymentCreated || view.AmountCents != o.TotalCents || view.PayURL == "" {
		t.Fatalf("view %+v", view)
	}
	// Opening the payment again is harmless: same payment.
	if again, err := f.svc.Pay(ctx, alice, o.No); err != nil || again.PaymentID != view.PaymentID || f.provider.count("create") != 2 {
		t.Fatalf("pay again: %+v %v", again, err)
	}

	f.provider.pay(view.PaymentID)
	if err := f.notify(t, view.PaymentID); err != nil {
		t.Fatal(err)
	}
	got := f.status(t, o.No)
	if got.Status != service.StatusPaid || got.PaidAt == nil {
		t.Fatalf("after notify: %s paidAt=%v", got.Status, got.PaidAt)
	}
	if f.inventory.count("commit") != 1 || f.inventory.calls[1].operation != "commit-"+o.No {
		t.Fatalf("commit calls: %+v", f.inventory.calls)
	}
	// The provider repeats notifications until acknowledged: no second commit.
	if err := f.notify(t, view.PaymentID); err != nil || f.inventory.count("commit") != 1 {
		t.Fatalf("duplicate notify: err=%v commits=%d", err, f.inventory.count("commit"))
	}
	// A paid order cannot be cancelled, and Pay just reports it.
	if _, err := f.svc.Cancel(ctx, alice, o.No); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("cancel paid: %v", err)
	}
	if paid, err := f.svc.Pay(ctx, alice, o.No); err != nil || paid.State != service.PaymentSucceeded || paid.PayURL != "" {
		t.Fatalf("pay a paid order: %+v %v", paid, err)
	}
}

func TestNotifyRejectsForgedAndStaleNotices(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID)

	body, _ := json.Marshal(utility.PaymentNotice{PaymentID: view.PaymentID, NotifyTime: f.clock()})
	if err := f.svc.Notify(ctx, body, "forged"); !errors.Is(err, service.ErrBadNotice) {
		t.Fatalf("forged signature: %v", err)
	}
	stale, _ := json.Marshal(utility.PaymentNotice{PaymentID: view.PaymentID, NotifyTime: f.clock().Add(-6 * time.Minute)})
	if err := f.svc.Notify(ctx, stale, "good"); !errors.Is(err, service.ErrBadNotice) {
		t.Fatalf("stale notice: %v", err)
	}
	if err := f.svc.Notify(ctx, []byte("not json"), "good"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("garbage: %v", err)
	}
	if err := f.notify(t, "P-unknown"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("unknown payment: %v", err)
	}
	if f.provider.count("query") != 0 || f.status(t, o.No).Status != service.StatusPendingPayment {
		t.Fatal("a rejected notice had an effect")
	}
}

func TestNotifyIsOnlyAHint(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	// The notice claims success; the provider says it is still unpaid.
	if err := f.notify(t, view.PaymentID); err != nil {
		t.Fatal(err)
	}
	if got := f.status(t, o.No); got.Status != service.StatusPendingPayment {
		t.Fatalf("believed the notice: %s", got.Status)
	}
	// Unreachable provider: not acknowledged, so the provider will retry.
	f.provider.pay(view.PaymentID)
	f.provider.fail["query"] = errUnavailable
	if err := f.notify(t, view.PaymentID); err == nil {
		t.Fatal("acknowledged without knowing")
	}
}

func TestPayRecoversALostNotification(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID) // paid, notification never arrives

	// The shopper presses "我已支付" (pays again): the provider says paid.
	again, err := f.svc.Pay(ctx, alice, o.No)
	if err != nil || again.State != service.PaymentSucceeded || again.Order.Status != service.StatusPaid {
		t.Fatalf("pay again: %+v %v", again, err)
	}
}

func TestCommitWithoutAnswerIsRetriedByTheWorker(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID)
	f.inventory.commits = []reply{{err: errUnavailable}}
	f.notify(t, view.PaymentID)
	if got := f.status(t, o.No); got.Status != service.StatusPaymentConfirming {
		t.Fatalf("after a lost commit: %s", got.Status)
	}
	f.svc.Recover(ctx)
	if got := f.status(t, o.No); got.Status != service.StatusPaid || f.inventory.count("commit") != 2 {
		t.Fatalf("after recovery: %s, commits %d", got.Status, f.inventory.count("commit"))
	}
}

func TestExpiryClosesThePaymentBeforeReleasing(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)

	f.advance(timing.PaymentTimeout)
	if _, err := f.svc.Pay(ctx, alice, o.No); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("paying an expired order: %v", err)
	}
	// The provider cannot be reached: the order waits in CLOSING, stock held.
	f.provider.fail["close"] = errUnavailable
	f.svc.Recover(ctx)
	if got := f.status(t, o.No); got.Status != service.StatusClosing || f.inventory.count("release") != 0 {
		t.Fatalf("released without knowing the payment: %s, releases %d", got.Status, f.inventory.count("release"))
	}
	f.svc.Recover(ctx)
	if got := f.status(t, o.No); got.Status != service.StatusClosed || f.inventory.count("release") != 1 {
		t.Fatalf("after recovery: %s, releases %d", got.Status, f.inventory.count("release"))
	}
	if f.provider.state(view.PaymentID) != service.PaymentClosed {
		t.Fatalf("provider: %s", f.provider.state(view.PaymentID))
	}
	if p, _ := f.payments.Find(ctx, view.PaymentID); p.State != service.PaymentClosed {
		t.Fatalf("local payment: %s", p.State)
	}
	// The payer is too late now.
	if f.provider.pay(view.PaymentID) {
		t.Fatal("paid a closed payment")
	}
}

func TestCancelLeavesATombstoneAtTheProvider(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	// Cancelled before the shopper ever opened the payment.
	if _, err := f.svc.Cancel(ctx, alice, o.No); err != nil {
		t.Fatal(err)
	}
	if f.provider.state("P"+o.No) != service.PaymentClosed {
		t.Fatal("no tombstone: a payment could still be created")
	}
	if _, err := f.svc.Pay(ctx, alice, o.No); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("pay a cancelled order: %v", err)
	}
}

func TestLatePaymentIsRefundedOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)

	// The shopper cancels; in the same instant the payer pays.
	f.provider.pay(view.PaymentID)
	got, err := f.svc.Cancel(ctx, alice, o.No)
	if err != nil || got.Status != service.StatusClosed {
		t.Fatalf("cancel: %+v %v", got, err)
	}
	r := f.payments.refund(view.PaymentID)
	if r == nil || r.Reason != service.RefundLatePayment || r.ID != "TRP"+o.No || r.AmountCents != o.TotalCents {
		t.Fatalf("refund: %+v", r)
	}
	if f.inventory.count("commit") != 0 || f.inventory.count("release") != 1 {
		t.Fatalf("stock: %+v", f.inventory.calls)
	}
	// The notification arrives too, twice: still one refund, no commit.
	f.notify(t, view.PaymentID)
	f.notify(t, view.PaymentID)
	// The refund call fails once, then goes through.
	f.provider.fail["refund"] = errUnavailable
	f.svc.Recover(ctx)
	if r := f.payments.refund(view.PaymentID); r.State != service.TechRefundPending || r.Attempts != 1 {
		t.Fatalf("after a failed refund: %+v", r)
	}
	f.svc.Recover(ctx)
	f.svc.Recover(ctx)
	if r := f.payments.refund(view.PaymentID); r.State != service.TechRefundSucceeded {
		t.Fatalf("refund not done: %+v", r)
	}
	if f.provider.state(view.PaymentID) != service.PaymentRefunded || f.provider.count("refund") != 2 {
		t.Fatalf("provider: %s after %d refund calls", f.provider.state(view.PaymentID), f.provider.count("refund"))
	}
	if got := f.status(t, o.No); got.Status != service.StatusClosed || got.CloseReason != service.ReasonUserCancelled {
		t.Fatalf("order reopened: %+v", got)
	}
}

func TestCommitRefusedRefundsThenCloses(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID)
	f.inventory.commits = []reply{{outcome: service.OutcomeStateConflict}}
	f.notify(t, view.PaymentID)

	got := f.status(t, o.No)
	if got.Status != service.StatusException || got.CloseReason != service.ReasonInventoryConflict {
		t.Fatalf("after a refused commit: %+v", got)
	}
	if r := f.payments.refund(view.PaymentID); r == nil || r.Reason != service.RefundCommitFailed {
		t.Fatalf("refund: %+v", r)
	}
	f.svc.Recover(ctx) // refund → CLOSING
	f.svc.Recover(ctx) // close the payment (already refunded), release → CLOSED
	if got := f.status(t, o.No); got.Status != service.StatusClosed {
		t.Fatalf("final: %s", got.Status)
	}
	if f.provider.state(view.PaymentID) != service.PaymentRefunded || f.provider.count("refund") != 1 {
		t.Fatalf("provider: %s, %d refunds", f.provider.state(view.PaymentID), f.provider.count("refund"))
	}
}

func TestMismatchedPaymentIsNotConfirmed(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	view, _ := f.svc.Pay(ctx, alice, o.No)
	f.provider.pay(view.PaymentID)
	f.provider.tamper = 1 // the provider reports one cent more
	if err := f.notify(t, view.PaymentID); !errors.Is(err, service.ErrPaymentMismatch) {
		t.Fatalf("notify: %v", err)
	}
	if got := f.status(t, o.No); got.Status != service.StatusPendingPayment || f.inventory.count("commit") != 0 {
		t.Fatalf("acted on a mismatched payment: %s", got.Status)
	}
}

func TestPayIsForTheOwnerOnly(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	if _, err := f.svc.Pay(ctx, bob, o.No); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("bob paying alice's order: %v", err)
	}
	f.provider.fail["create"] = errUnavailable
	if _, err := f.svc.Pay(ctx, alice, o.No); err == nil {
		t.Fatal("provider down, yet Pay succeeded")
	}
}

// The payer pays while the worker expires the order, many times over. Each
// order ends exactly one of two ways: paid with the stock sold and no
// refund, or closed with the stock released and, if the money arrived,
// exactly one refund. (The notification and the worker may both send the
// commit; it carries one operation id, so goods applies it once.)
func TestPaymentAndExpiryRace(t *testing.T) {
	ctx := context.Background()
	outcomes := map[string]int{}
	for i := 0; i < 100; i++ {
		f := newFixture()
		o := placed(t, f, "r1")
		view, _ := f.svc.Pay(ctx, alice, o.No)
		f.advance(timing.PaymentTimeout - time.Millisecond)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if f.provider.pay(view.PaymentID) {
				f.notify(t, view.PaymentID)
			}
		}()
		go func() {
			defer wg.Done()
			f.advance(time.Millisecond)
			f.svc.Recover(ctx)
		}()
		wg.Wait()
		for j := 0; j < 3; j++ {
			f.svc.Recover(ctx)
		}

		got := f.status(t, o.No)
		refund := f.payments.refund(view.PaymentID)
		outcomes[got.Status+map[bool]string{true: "+refund", false: ""}[refund != nil]]++
		paid := f.provider.state(view.PaymentID) != service.PaymentClosed
		switch got.Status {
		case service.StatusPaid:
			if refund != nil || f.inventory.operations("commit") != 1 || f.inventory.count("release") != 0 {
				t.Fatalf("run %d: paid, but refund=%+v calls=%+v", i, refund, f.inventory.calls)
			}
		case service.StatusClosed:
			if f.inventory.count("commit") != 0 || f.inventory.count("release") != 1 {
				t.Fatalf("run %d: closed, but calls=%+v", i, f.inventory.calls)
			}
			if paid != (refund != nil) || (refund != nil && refund.State != service.TechRefundSucceeded) {
				t.Fatalf("run %d: closed, paid=%v refund=%+v", i, paid, refund)
			}
		default:
			t.Fatalf("run %d: ended %s", i, got.Status)
		}
	}
	t.Logf("outcomes: %v", outcomes)
}

func TestPayableOrderSchedulesItsExpiryMessage(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	// The first reserve gets no answer; the worker's retry succeeds.
	f.inventory.reserves = []reply{{err: errUnavailable}}
	o, _ := f.svc.Create(ctx, alice, input("r1"))
	f.svc.Recover(ctx)
	f.svc.Recover(ctx)
	got := f.status(t, o.No)
	// One PENDING_PAYMENT change: its expiry message and order.placed.
	if len(f.orders.events) != 2 {
		t.Fatalf("%d events, want exactly two for one PENDING_PAYMENT change", len(f.orders.events))
	}
	e := f.orders.events[0]
	payload := e.Payload.(service.PaymentPending)
	if e.Type != service.EventPaymentPending || e.AggregateID != o.No || payload.OrderNo != o.No ||
		!payload.ExpiresAt.Equal(*got.ExpiresAt) || !e.DeliverAt.Equal(got.ExpiresAt.Add(service.ExpiryGrace)) {
		t.Fatalf("event %+v for order %+v", e, got)
	}
}

func TestPlacedOrderAnnouncesItsGoods(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	in := input("r1")
	in.FromCart = true
	o, err := f.svc.Create(ctx, alice, in)
	if err != nil {
		t.Fatal(err)
	}
	var placed []eventbus.Event
	for _, e := range f.orders.events {
		if e.Type == eventbus.TypeOrderPlaced {
			placed = append(placed, e)
		}
	}
	if len(placed) != 1 || placed[0].AggregateID != o.No || placed[0].DeliverAt != nil {
		t.Fatalf("order.placed events: %+v", placed)
	}
	payload := placed[0].Payload.(eventbus.OrderPlaced)
	// Merged quantities, the shopper, and the submit time (= created_at).
	want := map[uint64]uint32{1: 1, 2: 3}
	if payload.OrderNo != o.No || payload.UserID != alice || !payload.FromCart ||
		!payload.PlacedAt.Equal(o.CreatedAt) || len(payload.Items) != len(want) {
		t.Fatalf("payload %+v for order %+v", payload, o)
	}
	for _, item := range payload.Items {
		if want[item.GoodsID] != item.Quantity {
			t.Fatalf("items %+v, want %v", payload.Items, want)
		}
	}
}

func TestOrderWithoutStockIsNeverPlaced(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.inventory.reserves = []reply{{outcome: service.OutcomeInsufficientStock}}
	in := input("r1")
	in.FromCart = true
	o, _ := f.svc.Create(ctx, alice, in)
	if got := f.status(t, o.No); got.Status != service.StatusClosed || len(f.orders.events) != 0 {
		t.Fatalf("status %s, events %+v", got.Status, f.orders.events)
	}
}

func TestSettle(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o := placed(t, f, "r1")
	// Not expired yet: an early call changes nothing.
	if err := f.svc.Settle(ctx, o.No); err != nil || f.status(t, o.No).Status != service.StatusPendingPayment {
		t.Fatalf("settled a payable order: %v", err)
	}
	f.advance(timing.PaymentTimeout)
	if err := f.svc.Settle(ctx, o.No); err != nil {
		t.Fatal(err)
	}
	if got := f.status(t, o.No); got.Status != service.StatusClosed || got.CloseReason != service.ReasonPaymentTimeout || f.inventory.count("release") != 1 {
		t.Fatalf("after settle: %+v", got)
	}
	if err := f.svc.Settle(ctx, "nope"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("unknown order: %v", err)
	}
}
