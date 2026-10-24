package order

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

// ---- fakes ----------------------------------------------------------------

type fakeOrders struct {
	mu     sync.Mutex
	rows   map[uint64]*service.Order
	nextID uint64
	events []eventbus.Event // the outbox: appended only by winning changes
}

func newFakeOrders() *fakeOrders { return &fakeOrders{rows: map[uint64]*service.Order{}} }

func (f *fakeOrders) copyOf(o *service.Order) *service.Order {
	c := *o
	c.Items = append([]service.OrderItem(nil), o.Items...)
	return &c
}

func (f *fakeOrders) Create(_ context.Context, o *service.Order) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.UserID == o.UserID && row.RequestID == o.RequestID {
			return 0, service.ErrAlreadyExists
		}
	}
	f.nextID++
	stored := f.copyOf(o)
	stored.ID = f.nextID
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = time.Now()
	}
	f.rows[f.nextID] = stored
	return f.nextID, nil
}

func (f *fakeOrders) find(match func(*service.Order) bool) *service.Order {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if match(row) {
			return f.copyOf(row)
		}
	}
	return nil
}

func (f *fakeOrders) FindByNo(_ context.Context, no string) (*service.Order, error) {
	return f.find(func(o *service.Order) bool { return o.No == no }), nil
}

func (f *fakeOrders) FindByRequest(_ context.Context, userID uint64, requestID string) (*service.Order, error) {
	return f.find(func(o *service.Order) bool { return o.UserID == userID && o.RequestID == requestID }), nil
}

func (f *fakeOrders) NosByID(_ context.Context, ids []uint64) (map[uint64]string, error) {
	out := map[uint64]string{}
	for _, id := range ids {
		if o, ok := f.rows[id]; ok {
			out[id] = o.No
		}
	}
	return out, nil
}

func (f *fakeOrders) matching(flt *service.ListFilter) []*service.Order {
	var out []*service.Order
	for id := f.nextID; id > 0; id-- {
		o, ok := f.rows[id]
		if !ok || (flt.UserID != 0 && o.UserID != flt.UserID) || (flt.Status != "" && o.Status != flt.Status) {
			continue
		}
		out = append(out, f.copyOf(o))
	}
	return out
}

func (f *fakeOrders) List(_ context.Context, flt *service.ListFilter) ([]*service.Order, error) {
	all := f.matching(flt)
	start := (flt.Page - 1) * flt.Size
	if start >= len(all) {
		return nil, nil
	}
	return all[start:min(start+flt.Size, len(all))], nil
}

func (f *fakeOrders) Count(_ context.Context, flt *service.ListFilter) (int, error) {
	return len(f.matching(flt)), nil
}

func (f *fakeOrders) Transition(_ context.Context, id uint64, c service.Change) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o := f.rows[id]
	allowed := false
	for _, from := range c.From {
		allowed = allowed || o.Status == from
	}
	if !allowed {
		return false, nil
	}
	o.Status = c.To
	if c.CloseReason != "" {
		o.CloseReason = c.CloseReason
	}
	if c.ExpiresAt != nil {
		o.ExpiresAt = c.ExpiresAt
	}
	if c.PaidAt != nil {
		o.PaidAt = c.PaidAt
	}
	f.events = append(f.events, c.Events...)
	if c.ClosedAt != nil {
		o.ClosedAt = c.ClosedAt
	}
	if c.Shipment != nil {
		o.Carrier, o.TrackingNo = c.Shipment.Carrier, c.Shipment.TrackingNo
	}
	if c.ShippedAt != nil {
		o.ShippedAt = c.ShippedAt
	}
	if c.ConfirmDeadline != nil {
		o.ConfirmDeadline = c.ConfirmDeadline
	}
	if c.CompletedAt != nil {
		o.CompletedAt = c.CompletedAt
	}
	if c.CompleteReason != "" {
		o.CompleteReason = c.CompleteReason
	}
	return true, nil
}

func (f *fakeOrders) Due(_ context.Context, now time.Time, _ int) ([]*service.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*service.Order
	for id := uint64(1); id <= f.nextID; id++ {
		o := f.rows[id]
		switch {
		case o.Status == service.StatusReserving, o.Status == service.StatusClosing, o.Status == service.StatusPaymentConfirming,
			o.Status == service.StatusPendingPayment && o.ExpiresAt != nil && !o.ExpiresAt.After(now),
			o.Status == service.StatusShipped && o.ConfirmDeadline != nil && !o.ConfirmDeadline.After(now):
			out = append(out, f.copyOf(o))
		}
	}
	return out, nil
}

type fakeCatalog map[uint64]service.GoodsSnapshot

func (f fakeCatalog) Goods(_ context.Context, id uint64) (*service.GoodsSnapshot, error) {
	g, ok := f[id]
	if !ok {
		return nil, service.ErrNotFound
	}
	return &g, nil
}

type fakeAddresses map[uint64]service.Address

func (f fakeAddresses) Consignee(_ context.Context, id uint64) (*service.Address, error) {
	a, ok := f[id]
	if !ok {
		return nil, service.ErrNotFound
	}
	return &a, nil
}

type call struct {
	kind, operation, orderNo string
}

// fakeInventory answers with the next scripted reply (or SUCCEEDED) and
// records every call.
type fakeInventory struct {
	mu       sync.Mutex
	calls    []call
	reserves []reply
	commits  []reply
	releases []reply
}

type reply struct {
	outcome string
	err     error
}

func (f *fakeInventory) next(queue *[]reply) (string, error) {
	if len(*queue) == 0 {
		return service.OutcomeSucceeded, nil
	}
	r := (*queue)[0]
	*queue = (*queue)[1:]
	return r.outcome, r.err
}

func (f *fakeInventory) Reserve(_ context.Context, op, no string, _ []service.ItemInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{"reserve", op, no})
	return f.next(&f.reserves)
}

func (f *fakeInventory) Commit(_ context.Context, op, no string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{"commit", op, no})
	return f.next(&f.commits)
}

func (f *fakeInventory) Release(_ context.Context, op, no string, _ []service.ItemInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{"release", op, no})
	return f.next(&f.releases)
}

func (f *fakeInventory) count(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.kind == kind {
			n++
		}
	}
	return n
}

// ---- fixture --------------------------------------------------------------

type fixture struct {
	svc       service.IOrder
	orders    *fakeOrders
	payments  *fakePayments
	inventory *fakeInventory
	provider  *fakeProvider
	notices   *fakeNotices
	mu        sync.Mutex
	now       time.Time
}

func (f *fixture) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fixture) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

const alice, bob uint64 = 1, 2

var timing = service.Timing{ReserveTimeout: time.Minute, PaymentTimeout: 15 * time.Minute, AutoConfirm: 240 * time.Hour}

func newFixture() *fixture {
	f := &fixture{orders: newFakeOrders(), inventory: &fakeInventory{}, notices: &fakeNotices{}, now: time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)}
	f.payments = newFakePayments(f.orders)
	f.provider = newFakeProvider(f.clock)
	catalog := fakeCatalog{
		1: {Name: "电视", PriceCents: 199900, CoverURL: "https://img/tv.png"},
		2: {Name: "支架", PriceCents: 9900},
	}
	addresses := fakeAddresses{7: {Name: "张三", Phone: "13800000000", Address: "广东省东莞市 A101"}}
	f.svc = New(Deps{
		Orders: f.orders, Payments: f.payments, Catalog: catalog, Addresses: addresses,
		Inventory: f.inventory, Provider: f.provider, Notices: f.notices, Timing: timing, Now: f.clock,
	})
	return f
}

func input(requestID string) *service.CreateInput {
	return &service.CreateInput{
		RequestID:   requestID,
		Items:       []service.ItemInput{{GoodsID: 2, Quantity: 1}, {GoodsID: 1, Quantity: 1}, {GoodsID: 2, Quantity: 2}},
		ConsigneeID: 7,
		Remark:      " 放门口 ",
	}
}

// ---- tests ----------------------------------------------------------------

func TestCreateComputesPricesAndHoldsStock(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	o, err := f.svc.Create(ctx, alice, input("r1"))
	if err != nil {
		t.Fatal(err)
	}
	// 1 × 199900 + 3 × 9900, merged and priced by the server.
	if o.TotalCents != 229600 || len(o.Items) != 2 || o.Items[1].Quantity != 3 || o.Items[1].UnitPriceCents != 9900 {
		t.Fatalf("pricing: total=%d items=%+v", o.TotalCents, o.Items)
	}
	if o.ConsigneeName != "张三" || o.Remark != "放门口" {
		t.Fatalf("snapshot: %+v", o)
	}
	if o.Status != service.StatusPendingPayment || !o.ExpiresAt.Equal(f.now.Add(15*time.Minute)) {
		t.Fatalf("status=%s expires=%v", o.Status, o.ExpiresAt)
	}
	if c := f.inventory.calls[0]; c.kind != "reserve" || c.operation != "reserve-"+o.No || c.orderNo != o.No {
		t.Fatalf("reserve call: %+v", c)
	}
	if !strings.HasPrefix(o.No, "O20261020100000") || len(o.No) != 23 {
		t.Fatalf("order number %q", o.No)
	}
}

func TestCreateIsIdempotentPerRequestID(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	first, _ := f.svc.Create(ctx, alice, input("r1"))

	// The same submit again, items in another order: same order, no new reserve.
	retry := input("r1")
	retry.Items = []service.ItemInput{{GoodsID: 1, Quantity: 1}, {GoodsID: 2, Quantity: 3}}
	again, err := f.svc.Create(ctx, alice, retry)
	if err != nil || again.No != first.No || f.inventory.count("reserve") != 1 {
		t.Fatalf("retry: no=%s err=%v reserves=%d", again.No, err, f.inventory.count("reserve"))
	}
	// Same id, different content: refused.
	changed := input("r1")
	changed.Items = []service.ItemInput{{GoodsID: 1, Quantity: 2}}
	if _, err := f.svc.Create(ctx, alice, changed); !errors.Is(err, service.ErrIdempotencyConflict) {
		t.Fatalf("changed payload: err = %v", err)
	}
	// Another shopper may use the same request id.
	if o, err := f.svc.Create(ctx, bob, input("r1")); err != nil || o.No == first.No {
		t.Fatalf("other user: %v %v", o, err)
	}
}

func TestCreateRefusesUnknownGoodsAndOthersAddresses(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	in := input("r1")
	in.Items = append(in.Items, service.ItemInput{GoodsID: 99, Quantity: 1})
	if _, err := f.svc.Create(ctx, alice, in); !errors.Is(err, service.ErrGoodsUnavailable) {
		t.Fatalf("unknown goods: err = %v", err)
	}
	in = input("r2")
	in.ConsigneeID = 8 // not in alice's address book
	if _, err := f.svc.Create(ctx, alice, in); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("foreign address: err = %v", err)
	}
	if len(f.orders.rows) != 0 || len(f.inventory.calls) != 0 {
		t.Fatal("a refused request left an order or touched stock")
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	tooMany := make([]service.ItemInput, 21)
	for i := range tooMany {
		tooMany[i] = service.ItemInput{GoodsID: uint64(i + 1), Quantity: 1}
	}
	bad := map[string]func(*service.CreateInput){
		"no request id":     func(in *service.CreateInput) { in.RequestID = "" },
		"padded request id": func(in *service.CreateInput) { in.RequestID = " r " },
		"long request id":   func(in *service.CreateInput) { in.RequestID = strings.Repeat("x", 65) },
		"no items":          func(in *service.CreateInput) { in.Items = nil },
		"zero quantity":     func(in *service.CreateInput) { in.Items[0].Quantity = 0 },
		"merged over 99": func(in *service.CreateInput) {
			in.Items = []service.ItemInput{{GoodsID: 1, Quantity: 60}, {GoodsID: 1, Quantity: 40}}
		},
		"more than 20 lines": func(in *service.CreateInput) { in.Items = tooMany },
		"no address":         func(in *service.CreateInput) { in.ConsigneeID = 0 },
		"remark over 255":    func(in *service.CreateInput) { in.Remark = strings.Repeat("备", 256) },
	}
	for name, edit := range bad {
		in := input("r")
		edit(in)
		if _, err := f.svc.Create(ctx, alice, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestRefusedReservationClosesTheOrderWithoutRelease(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.inventory.reserves = []reply{{outcome: service.OutcomeInsufficientStock}}

	o, err := f.svc.Create(ctx, alice, input("r1"))
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != service.StatusClosed || o.CloseReason != service.ReasonInsufficientStock || o.ClosedAt == nil {
		t.Fatalf("got %s/%s", o.Status, o.CloseReason)
	}
	if f.inventory.count("release") != 0 {
		t.Fatal("released stock that was never reserved")
	}
}

func TestUnknownReserveOutcomeIsRetriedByTheWorker(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.inventory.reserves = []reply{{err: errors.New("deadline exceeded")}}

	o, err := f.svc.Create(ctx, alice, input("r1"))
	if err != nil {
		t.Fatalf("a timeout must not fail the submit: %v", err)
	}
	if o.Status != service.StatusReserving {
		t.Fatalf("status = %s, want RESERVING", o.Status)
	}
	f.advance(5 * time.Second)
	if err := f.svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	o, _ = f.svc.Get(ctx, o.No)
	if o.Status != service.StatusPendingPayment {
		t.Fatalf("after retry: %s", o.Status)
	}
	// Both attempts used the same operation id, so the goods service acted once.
	if a, b := f.inventory.calls[0], f.inventory.calls[1]; a.operation != b.operation {
		t.Fatalf("retry used a new operation id: %s vs %s", a.operation, b.operation)
	}
}

func TestReservingPastTheDeadlineClosesAndReleases(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.inventory.reserves = []reply{{err: errors.New("unavailable")}}
	o, _ := f.svc.Create(ctx, alice, input("r1"))

	f.advance(timing.ReserveTimeout + time.Second)
	_ = f.svc.Recover(ctx)
	o, _ = f.svc.Get(ctx, o.No)
	if o.Status != service.StatusClosed || o.CloseReason != service.ReasonReserveTimeout {
		t.Fatalf("got %s/%s", o.Status, o.CloseReason)
	}
	// The release leaves a tombstone, so a reserve still in flight is refused.
	if f.inventory.count("release") != 1 {
		t.Fatal("no release for an order that may hold stock")
	}
}

func TestUnpaidOrdersExpireAndReleaseOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o, _ := f.svc.Create(ctx, alice, input("r1"))

	f.advance(14 * time.Minute)
	_ = f.svc.Recover(ctx)
	if got, _ := f.svc.Get(ctx, o.No); got.Status != service.StatusPendingPayment {
		t.Fatalf("closed early: %s", got.Status)
	}
	f.advance(2 * time.Minute)
	_ = f.svc.Recover(ctx)
	_ = f.svc.Recover(ctx)
	got, _ := f.svc.Get(ctx, o.No)
	if got.Status != service.StatusClosed || got.CloseReason != service.ReasonPaymentTimeout {
		t.Fatalf("got %s/%s", got.Status, got.CloseReason)
	}
	if f.inventory.count("release") != 1 {
		t.Fatalf("released %d times", f.inventory.count("release"))
	}
}

func TestCancel(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o, _ := f.svc.Create(ctx, alice, input("r1"))

	if _, err := f.svc.Cancel(ctx, bob, o.No); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("someone else's order: err = %v", err)
	}
	// The release fails once: the order waits in CLOSING for the worker.
	f.inventory.releases = []reply{{err: errors.New("unavailable")}}
	got, err := f.svc.Cancel(ctx, alice, o.No)
	if err != nil || got.Status != service.StatusClosing || got.CloseReason != service.ReasonUserCancelled {
		t.Fatalf("cancel: %+v %v", got, err)
	}
	_ = f.svc.Recover(ctx)
	got, _ = f.svc.Get(ctx, o.No)
	if got.Status != service.StatusClosed {
		t.Fatalf("after worker: %s", got.Status)
	}
	// Cancelling again is harmless and does not release again.
	before := f.inventory.count("release")
	if again, err := f.svc.Cancel(ctx, alice, o.No); err != nil || again.Status != service.StatusClosed {
		t.Fatalf("cancel twice: %+v %v", again, err)
	}
	if f.inventory.count("release") != before {
		t.Fatal("a second cancel released again")
	}
}

// The shopper cancels just as the order expires: exactly one of them wins the
// conditional change, and the stock is released exactly once. (The worker may
// also find the order CLOSING and send the release again; it carries the same
// operation id, so goods applies it once.)
func TestCancelAndExpiryRaceReleasesOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o, _ := f.svc.Create(ctx, alice, input("r1"))
	f.advance(timing.PaymentTimeout + time.Second)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = f.svc.Cancel(ctx, alice, o.No) }()
	go func() { defer wg.Done(); _ = f.svc.Recover(ctx) }()
	wg.Wait()

	got, _ := f.svc.Get(ctx, o.No)
	if got.Status != service.StatusClosed || f.inventory.operations("release") != 1 || got.CloseReason == "" {
		t.Fatalf("status=%s releases=%+v", got.Status, f.inventory.calls)
	}
}

// A cancel that lands while the reservation is in flight: the goods service
// answers ALREADY_RELEASED, and the order still ends closed and released.
func TestReserveAnsweredAlreadyReleased(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.inventory.reserves = []reply{{outcome: service.OutcomeAlreadyReleased}}
	o, _ := f.svc.Create(ctx, alice, input("r1"))
	if o.Status != service.StatusClosed || o.CloseReason != service.ReasonReserveConflict {
		t.Fatalf("got %s/%s", o.Status, o.CloseReason)
	}
}

func TestPaidOrdersCannotBeCancelled(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	o, _ := f.svc.Create(ctx, alice, input("r1"))
	f.orders.rows[o.ID].Status = service.StatusPaid
	if _, err := f.svc.Cancel(ctx, alice, o.No); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("err = %v", err)
	}
}

func TestListsAreScoped(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	_, _ = f.svc.Create(ctx, alice, input("a1"))
	_, _ = f.svc.Create(ctx, alice, input("a2"))
	_, _ = f.svc.Create(ctx, bob, input("b1"))

	mine, total, err := f.svc.ListMine(ctx, alice, "", 1, 10)
	if err != nil || total != 2 || len(mine) != 2 {
		t.Fatalf("mine: total=%d err=%v", total, err)
	}
	for _, o := range mine {
		if o.UserID != alice {
			t.Fatalf("leaked %+v", o)
		}
	}
	if _, all, _ := f.svc.List(ctx, &service.ListFilter{Page: 1, Size: 10}); all != 3 {
		t.Fatalf("admin list total = %d", all)
	}
	if _, _, err := f.svc.ListMine(ctx, 0, "", 1, 10); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatal("user id 0 would list everyone")
	}
}

func TestPriceOverflowIsRefused(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.svc = New(Deps{
		Orders: f.orders, Payments: f.payments, Catalog: fakeCatalog{1: {Name: "x", PriceCents: ^uint64(0) / 2}},
		Addresses: fakeAddresses{7: {}}, Inventory: f.inventory, Provider: f.provider, Notices: f.notices,
		Timing: timing, Now: f.clock,
	})
	in := &service.CreateInput{RequestID: "r", Items: []service.ItemInput{{GoodsID: 1, Quantity: 3}}, ConsigneeID: 7}
	if _, err := f.svc.Create(ctx, alice, in); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("err = %v", err)
	}
}
