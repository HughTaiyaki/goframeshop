package refund

import (
	"context"
	"errors"
	"strings"
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
	for _, row := range f.rows {
		if row.No == no {
			return row, nil
		}
	}
	return nil, nil
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

func setup() (service.IRefund, *fakeOrders) {
	orders := &fakeOrders{byNo: map[string]*service.Order{
		"PAID-1":    {ID: 1, No: "PAID-1", UserID: alice, Status: service.StatusPaid, Items: []service.OrderItem{{GoodsID: 10}, {GoodsID: 11}}},
		"PENDING-2": {ID: 2, No: "PENDING-2", UserID: alice, Status: service.StatusPendingPayment, Items: []service.OrderItem{{GoodsID: 10}}},
	}}
	return New(&fakeRefunds{}, orders, func() time.Time { return time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC) }), orders
}

func TestOnlyPaidOrdersOfTheCallerCanBeRefunded(t *testing.T) {
	ctx := context.Background()
	svc, _ := setup()

	r, err := svc.Create(ctx, alice, "PAID-1", 10, " 不想要了 ")
	if err != nil || r.Status != service.RefundPending || r.Reason != "不想要了" || r.OrderNo != "PAID-1" || !strings.HasPrefix(r.No, "R") {
		t.Fatalf("create: %+v %v", r, err)
	}
	cases := map[string]struct {
		user    uint64
		orderNo string
		goods   uint64
		reason  string
		want    error
	}{
		"not paid yet":         {alice, "PENDING-2", 10, "x", service.ErrFailedPrecondition},
		"someone else's order": {bob, "PAID-1", 10, "x", service.ErrNotFound},
		"unknown order":        {alice, "NOPE", 10, "x", service.ErrNotFound},
		"goods not in order":   {alice, "PAID-1", 99, "x", service.ErrInvalidArgument},
		"blank reason":         {alice, "PAID-1", 11, "  ", service.ErrInvalidArgument},
		"reason too long":      {alice, "PAID-1", 11, strings.Repeat("因", 256), service.ErrInvalidArgument},
		"second request":       {alice, "PAID-1", 10, "again", service.ErrAlreadyExists},
	}
	for name, c := range cases {
		if _, err := svc.Create(ctx, c.user, c.orderNo, c.goods, c.reason); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}

	if _, err := svc.GetMine(ctx, bob, r.No); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("other user's refund: err = %v", err)
	}
	if mine, total, _ := svc.ListMine(ctx, bob, 1, 10); total != 0 || len(mine) != 0 {
		t.Fatalf("bob sees %d refunds", total)
	}
	if _, total, _ := svc.List(ctx, service.RefundPending, 1, 10); total != 1 {
		t.Fatalf("admin list total = %d", total)
	}
}
