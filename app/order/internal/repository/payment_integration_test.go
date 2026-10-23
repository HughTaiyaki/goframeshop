package repository_test

// Integration tests for the payment repository against a real MySQL: that
// ConfirmPaid's row lock really serialises "payment arrived" against a
// concurrent close of the same order, and that refunds are scheduled once.
//
//	make infra-up && make db-init
//	GOFRAMESHOP_IT=1 go test ./app/order/internal/repository/ -run Integration -v
//
// ORDERS_IT_LINK overrides the connection. Every test deletes its own rows.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

func integration(t *testing.T) context.Context {
	t.Helper()
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL")
	}
	link := os.Getenv("ORDERS_IT_LINK")
	if link == "" {
		link = "mysql:shop:change-me@tcp(127.0.0.1:3306)/orders?loc=Local&parseTime=true"
	}
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link, MaxOpenConnCount: 32}}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := dao.OrderInfo.DB().Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("cannot reach MySQL: %v", err)
	}
	return ctx
}

// pendingOrder inserts a PENDING_PAYMENT order of 100 cents and removes it,
// its lines, payment and refund afterwards.
func pendingOrder(t *testing.T, ctx context.Context, n int) *service.Order {
	t.Helper()
	no := fmt.Sprintf("IT%d%03d", time.Now().UnixNano()%1e15, n)
	expires := time.Now().Add(time.Minute)
	o := &service.Order{
		No: no, UserID: 999999, RequestID: no, RequestHash: "it", Status: service.StatusPendingPayment,
		TotalCents: 100, ReserveDeadline: time.Now(), ExpiresAt: &expires,
		Items: []service.OrderItem{{GoodsID: 1, GoodsName: "it", UnitPriceCents: 100, Quantity: 1, SubtotalCents: 100}},
	}
	id, err := repository.NewOrder().Create(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	o.ID = id
	t.Cleanup(func() {
		dao.TechRefund.Ctx(ctx).Where(dao.TechRefund.Columns().OrderId, id).Delete()
		dao.Payment.Ctx(ctx).Where(dao.Payment.Columns().OrderId, id).Delete()
		dao.OrderGoods.Ctx(ctx).Where(dao.OrderGoods.Columns().OrderId, id).Delete()
		dao.OrderInfo.Ctx(ctx).Where(dao.OrderInfo.Columns().Id, id).Delete()
	})
	return o
}

func TestIntegrationPaymentAgainstClose(t *testing.T) {
	ctx := integration(t)
	orders, payments := repository.NewOrder(), repository.NewPayment()
	seen := map[string]int{}
	for i := 0; i < 30; i++ {
		o := pendingOrder(t, ctx, i)
		paymentID := service.PaymentID(o.No)
		if err := payments.Create(ctx, &service.Payment{ID: paymentID, OrderID: o.ID, AmountCents: 100, State: service.PaymentCreated}); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var outcome string
		var closed bool
		var errPaid, errClose error
		wg.Add(2)
		go func() {
			defer wg.Done()
			outcome, errPaid = payments.ConfirmPaid(ctx, o.ID, &service.Payment{
				ID: paymentID, OrderID: o.ID, AmountCents: 100, ProviderTxID: "T" + o.No,
			}, time.Now())
		}()
		go func() {
			defer wg.Done()
			closed, errClose = orders.Transition(ctx, o.ID, service.Change{
				From: []string{service.StatusPendingPayment}, To: service.StatusClosing, CloseReason: service.ReasonPaymentTimeout,
			})
		}()
		wg.Wait()
		if errPaid != nil || errClose != nil {
			t.Fatal(errPaid, errClose)
		}
		got, _ := orders.FindByNo(ctx, o.No)
		refunds, _ := dao.TechRefund.Ctx(ctx).Where(dao.TechRefund.Columns().OrderId, o.ID).Count()
		payment, _ := payments.Find(ctx, paymentID)
		if payment.State != service.PaymentSucceeded || payment.ProviderTxID != "T"+o.No {
			t.Fatalf("payment not recorded: %+v", payment)
		}
		switch outcome {
		case service.PaidConfirming:
			if closed || got.Status != service.StatusPaymentConfirming || got.PaidAt == nil || refunds != 0 {
				t.Fatalf("payment won, yet closed=%v status=%s refunds=%d", closed, got.Status, refunds)
			}
		case service.PaidLate:
			if !closed || got.Status != service.StatusClosing || refunds != 1 {
				t.Fatalf("close won, yet closed=%v status=%s refunds=%d", closed, got.Status, refunds)
			}
		default:
			t.Fatalf("outcome %q", outcome)
		}
		seen[outcome]++
		// Repeating the confirmation changes nothing.
		again, _ := payments.ConfirmPaid(ctx, o.ID, &service.Payment{ID: paymentID, OrderID: o.ID, AmountCents: 100, ProviderTxID: "T" + o.No}, time.Now())
		after, _ := dao.TechRefund.Ctx(ctx).Where(dao.TechRefund.Columns().OrderId, o.ID).Count()
		if after != refunds || again != service.PaidDuplicate {
			t.Fatalf("repeat: %s, refunds %d -> %d", again, refunds, after)
		}
	}
	t.Logf("outcomes: %v", seen)
}

func TestIntegrationExceptionRefundCloses(t *testing.T) {
	ctx := integration(t)
	orders, payments := repository.NewOrder(), repository.NewPayment()
	o := pendingOrder(t, ctx, 0)
	paymentID := service.PaymentID(o.No)
	if _, err := payments.ConfirmPaid(ctx, o.ID, &service.Payment{ID: paymentID, OrderID: o.ID, AmountCents: 100, ProviderTxID: "T" + o.No}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if changed, err := payments.Except(ctx, o.ID, paymentID); err != nil || !changed {
		t.Fatalf("except: %v %v", changed, err)
	}
	if changed, _ := payments.Except(ctx, o.ID, paymentID); changed {
		t.Fatal("excepted twice")
	}
	due, err := payments.DueRefunds(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var refund *service.TechRefund
	for _, r := range due {
		if r.OrderID == o.ID {
			refund = r
		}
	}
	if refund == nil || refund.Reason != service.RefundCommitFailed || refund.ID != service.TechRefundID(paymentID) || refund.AmountCents != 100 {
		t.Fatalf("refund: %+v", refund)
	}
	if err := payments.RefundAttempted(ctx, refund.ID); err != nil {
		t.Fatal(err)
	}
	if err := payments.Refunded(ctx, refund.ID); err != nil {
		t.Fatal(err)
	}
	if err := payments.Refunded(ctx, refund.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := orders.FindByNo(ctx, o.No)
	payment, _ := payments.Find(ctx, paymentID)
	if got.Status != service.StatusClosing || got.CloseReason != service.ReasonInventoryConflict || payment.State != service.PaymentRefunded {
		t.Fatalf("after the refund: order %s/%s, payment %s", got.Status, got.CloseReason, payment.State)
	}
	// A late confirmation must not turn a refunded payment back into SUCCEEDED,
	// nor schedule a second refund.
	if outcome, _ := payments.ConfirmPaid(ctx, o.ID, &service.Payment{ID: paymentID, OrderID: o.ID, AmountCents: 100, ProviderTxID: "T" + o.No}, time.Now()); outcome != service.PaidDuplicate {
		t.Fatalf("confirm on a closing order: %s", outcome)
	}
	if payment, _ := payments.Find(ctx, paymentID); payment.State != service.PaymentRefunded {
		t.Fatalf("refunded payment downgraded to %s", payment.State)
	}
}
