package refund

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

type fakeOrders struct {
	service.OrderRepository
	byNo map[string]*service.Order
}

func (f *fakeOrders) FindByNo(_ context.Context, no string) (*service.Order, error) {
	return f.byNo[no], nil
}

type fakeRefunds struct {
	mu   sync.Mutex
	rows []*service.Refund
}

func (f *fakeRefunds) Create(_ context.Context, r *service.Refund) (uint64, error) {
	for _, row := range f.rows {
		if row.OrderID == r.OrderID && row.GoodsID == r.GoodsID {
			return 0, service.ErrAlreadyExists
		}
	}
	stored := *r
	stored.ID = uint64(len(f.rows) + 1)
	f.rows = append(f.rows, &stored)
	return stored.ID, nil
}

func (f *fakeRefunds) FindByNo(_ context.Context, no string) (*service.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.No == no {
			c := *row
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeRefunds) ByOrder(_ context.Context, orderID uint64) ([]*service.Refund, error) {
	var out []*service.Refund
	for _, row := range f.rows {
		if row.OrderID == orderID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeRefunds) Transition(_ context.Context, id uint64, c service.RefundChange) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.ID != id || row.Status != c.From {
			continue
		}
		row.Status = c.To
		if c.ReviewNote != nil {
			row.ReviewNote = *c.ReviewNote
		}
		if c.ReviewedAt != nil {
			row.ReviewedAt = c.ReviewedAt
		}
		if c.RefundedAt != nil {
			row.RefundedAt = c.RefundedAt
		}
		return true, nil
	}
	return false, nil
}

func (f *fakeRefunds) DueApproved(_ context.Context, _ int) ([]*service.Refund, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*service.Refund
	for _, row := range f.rows {
		if row.Status == service.RefundApproved {
			c := *row
			out = append(out, &c)
		}
	}
	return out, nil
}

func (f *fakeRefunds) Attempted(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.ID == id {
			row.Attempts++
		}
	}
	return nil
}

// fakeProvider records refunds by id; fail makes the next calls error out
// (no answer), as a timeout would.
type fakeProvider struct {
	service.Provider
	mu      sync.Mutex
	fail    bool
	refunds map[string]uint64 // refund id -> amount
	calls   int
}

func (p *fakeProvider) Refund(_ context.Context, refundID, paymentID string, amount uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.fail {
		return errors.New("unavailable")
	}
	if p.refunds == nil {
		p.refunds = map[string]uint64{}
	}
	p.refunds[refundID] = amount
	return nil
}

func (f *fakeRefunds) List(_ context.Context, userID uint64, status string, _, _ int) ([]*service.Refund, error) {
	var out []*service.Refund
	for _, row := range f.rows {
		if (userID == 0 || row.UserID == userID) && (status == "" || row.Status == status) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeRefunds) Count(ctx context.Context, userID uint64, status string) (int, error) {
	rows, _ := f.List(ctx, userID, status, 0, 0)
	return len(rows), nil
}

const alice, bob uint64 = 1, 2

var (
	today     = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	window    = 7 * 24 * time.Hour
	doneEarly = today.Add(-window - time.Second)
	doneJust  = today.Add(-window)
)

func setup() (service.IRefund, *fakeOrders) {
	svc, orders, _, _ := setupAll()
	return svc, orders
}

func setupAll() (service.IRefund, *fakeOrders, *fakeRefunds, *fakeProvider) {
	orders := &fakeOrders{byNo: map[string]*service.Order{
		"PAID-1": {ID: 1, No: "PAID-1", UserID: alice, Status: service.StatusPaid,
			Items: []service.OrderItem{{GoodsID: 10, SubtotalCents: 39800}, {GoodsID: 11, SubtotalCents: 990}}},
		"OLD-6":     {ID: 6, No: "OLD-6", UserID: alice, Status: service.StatusCompleted, CompletedAt: &doneEarly, Items: []service.OrderItem{{GoodsID: 10}}},
		"EDGE-7":    {ID: 7, No: "EDGE-7", UserID: alice, Status: service.StatusCompleted, CompletedAt: &doneJust, Items: []service.OrderItem{{GoodsID: 10}}},
		"PENDING-2": {ID: 2, No: "PENDING-2", UserID: alice, Status: service.StatusPendingPayment, Items: []service.OrderItem{{GoodsID: 10}}},
		"SHIPPED-3": {ID: 3, No: "SHIPPED-3", UserID: alice, Status: service.StatusShipped, Items: []service.OrderItem{{GoodsID: 10}}},
		"DONE-4":    {ID: 4, No: "DONE-4", UserID: alice, Status: service.StatusCompleted, Items: []service.OrderItem{{GoodsID: 10}}},
		"CLOSED-5":  {ID: 5, No: "CLOSED-5", UserID: alice, Status: service.StatusClosed, Items: []service.OrderItem{{GoodsID: 10}}},
	}}
	refunds, provider := &fakeRefunds{}, &fakeProvider{}
	svc := New(refunds, orders, provider, service.RefundSettings{AfterSaleWindow: window}, func() time.Time { return today })
	return svc, orders, refunds, provider
}

func TestOnlyPaidOrdersOfTheCallerCanBeRefunded(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup()

	r, err := svc.Create(ctx, alice, "PAID-1", 10, " 不想要了 ")
	if err != nil || r.Status != service.RefundPending || r.Reason != "不想要了" || r.OrderNo != "PAID-1" || !strings.HasPrefix(r.No, "R") ||
		r.AmountCents != 39800 {
		t.Fatalf("create: %+v %v", r, err)
	}
	cases := map[string]struct {
		user    uint64
		orderNo string
		goods   uint64
		reason  string
		want    error
	}{
		"not paid yet":           {alice, "PENDING-2", 10, "x", service.ErrFailedPrecondition},
		"closed":                 {alice, "CLOSED-5", 10, "x", service.ErrFailedPrecondition},
		"completed too long ago": {alice, "OLD-6", 10, "x", service.ErrFailedPrecondition},
		"someone else's order":   {bob, "PAID-1", 10, "x", service.ErrNotFound},
		"unknown order":          {alice, "NOPE", 10, "x", service.ErrNotFound},
		"goods not in order":     {alice, "PAID-1", 99, "x", service.ErrInvalidArgument},
		"blank reason":           {alice, "PAID-1", 11, "  ", service.ErrInvalidArgument},
		"reason too long":        {alice, "PAID-1", 11, strings.Repeat("因", 256), service.ErrInvalidArgument},
		"second request":         {alice, "PAID-1", 10, "again", service.ErrAlreadyExists},
	}
	for name, c := range cases {
		if _, err := svc.Create(ctx, c.user, c.orderNo, c.goods, c.reason); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}

	// Shipping does not take away the right to an after-sale (task 0017).
	for _, no := range []string{"SHIPPED-3", "DONE-4", "EDGE-7"} {
		if _, err := svc.Create(ctx, alice, no, 10, "坏了"); err != nil {
			t.Errorf("%s: %v", no, err)
		}
	}

	if _, err := svc.GetMine(ctx, bob, r.No); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("other user's refund: err = %v", err)
	}
	if mine, total, _ := svc.ListMine(ctx, bob, 1, 10); total != 0 || len(mine) != 0 {
		t.Fatalf("bob sees %d refunds", total)
	}
	if _, total, _ := svc.List(ctx, service.RefundPending, 1, 10); total != 4 {
		t.Fatalf("admin list total = %d", total)
	}
}
