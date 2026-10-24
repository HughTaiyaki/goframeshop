// Package refund handles after-sale requests: the shopper asks for one goods
// line of a paid order, an administrator approves or rejects, and an
// approved line's money is paid back through the provider (task 0018).
//
// The original tutorial wrote whatever the client sent. Here a request must
// name a paid order that belongs to the caller, and goods that are in it;
// the amount is that line's subtotal, worked out by the server.
//
//	PENDING ──Approve──▶ APPROVED ──provider refunded──▶ REFUNDED
//	   └────Reject────▶ REJECTED
//
// Every move is a conditional UPDATE, so an approve racing a reject, or two
// administrators clicking at once, decide the request exactly once. The
// provider call happens after the change commits, outside any transaction,
// with a refund id fixed per request: retrying it is always safe.
package refund

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

type sRefund struct {
	refunds  service.RefundRepository
	orders   service.OrderRepository
	provider service.Provider
	settings service.RefundSettings
	now      func() time.Time
}

func New(refunds service.RefundRepository, orders service.OrderRepository, provider service.Provider,
	settings service.RefundSettings, now func() time.Time) service.IRefund {
	return &sRefund{refunds: refunds, orders: orders, provider: provider, settings: settings, now: now}
}

func (s *sRefund) Create(ctx context.Context, userID uint64, orderNo string, goodsID uint64, reason string) (*service.Refund, error) {
	reason = strings.TrimSpace(reason)
	if userID == 0 || goodsID == 0 || reason == "" || utf8.RuneCountInString(reason) > service.MaxReasonRunes {
		return nil, service.ErrInvalidArgument
	}
	order, err := s.orders.FindByNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order == nil || order.UserID != userID {
		return nil, service.ErrNotFound
	}
	// Only money that was paid can be refunded: PAID, and also after
	// shipping (SHIPPED, COMPLETED), but not long after completion.
	if !service.Paid(order.Status) {
		return nil, service.ErrFailedPrecondition
	}
	if order.Status == service.StatusCompleted && order.CompletedAt != nil &&
		s.now().After(order.CompletedAt.Add(s.settings.AfterSaleWindow)) {
		return nil, service.ErrFailedPrecondition
	}
	var line *service.OrderItem
	for i := range order.Items {
		if order.Items[i].GoodsID == goodsID {
			line = &order.Items[i]
		}
	}
	if line == nil {
		return nil, service.ErrInvalidArgument
	}
	refund := &service.Refund{
		No:          newRefundNo(s.now()),
		OrderID:     order.ID,
		OrderNo:     order.No,
		UserID:      userID,
		GoodsID:     goodsID,
		Reason:      reason,
		Status:      service.RefundPending,
		AmountCents: line.SubtotalCents,
	}
	if refund.ID, err = s.refunds.Create(ctx, refund); err != nil {
		return nil, err
	}
	return s.refunds.FindByNo(ctx, refund.No)
}

func (s *sRefund) GetMine(ctx context.Context, userID uint64, refundNo string) (*service.Refund, error) {
	refund, err := s.refunds.FindByNo(ctx, refundNo)
	if err != nil {
		return nil, err
	}
	if refund == nil || refund.UserID != userID {
		return nil, service.ErrNotFound
	}
	return refund, nil
}

func (s *sRefund) ListMine(ctx context.Context, userID uint64, page, size int) ([]*service.Refund, int, error) {
	if userID == 0 {
		return nil, 0, service.ErrInvalidArgument
	}
	return s.list(ctx, userID, "", page, size)
}

func (s *sRefund) List(ctx context.Context, status string, page, size int) ([]*service.Refund, int, error) {
	return s.list(ctx, 0, status, page, size)
}

func (s *sRefund) list(ctx context.Context, userID uint64, status string, page, size int) ([]*service.Refund, int, error) {
	if page < 1 || size < 1 || size > 50 {
		return nil, 0, service.ErrInvalidArgument
	}
	total, err := s.refunds.Count(ctx, userID, status)
	if err != nil {
		return nil, 0, err
	}
	refunds, err := s.refunds.List(ctx, userID, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	return refunds, total, nil
}

func (s *sRefund) Approve(ctx context.Context, refundNo, note string) (*service.Refund, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > service.MaxReasonRunes {
		return nil, service.ErrInvalidArgument
	}
	refund, err := s.find(ctx, refundNo)
	if err != nil {
		return nil, err
	}
	if refund.Status == service.RefundPending {
		now := s.now()
		changed, err := s.refunds.Transition(ctx, refund.ID, service.RefundChange{
			From: service.RefundPending, To: service.RefundApproved, ReviewNote: &note, ReviewedAt: &now,
		})
		if err != nil {
			return nil, err
		}
		if changed {
			g.Log().Infof(ctx, "after-sale %s approved: %d cents for goods %d of order %s",
				refund.No, refund.AmountCents, refund.GoodsID, refund.OrderNo)
		}
		if refund, err = s.find(ctx, refundNo); err != nil {
			return nil, err
		}
	}
	switch refund.Status {
	case service.RefundApproved:
		// Ours or someone else's approval: either way the money is due. A
		// failure is not the caller's error: the request is approved, and
		// the worker keeps paying (logged by pay).
		_ = s.pay(ctx, refund)
		return s.find(ctx, refundNo)
	case service.RefundRefunded:
		return refund, nil
	default:
		return nil, service.ErrFailedPrecondition // rejected
	}
}

func (s *sRefund) Reject(ctx context.Context, refundNo, note string) (*service.Refund, error) {
	note = strings.TrimSpace(note)
	if note == "" || utf8.RuneCountInString(note) > service.MaxReasonRunes {
		return nil, service.ErrInvalidArgument
	}
	refund, err := s.find(ctx, refundNo)
	if err != nil {
		return nil, err
	}
	if refund.Status == service.RefundPending {
		now := s.now()
		if _, err := s.refunds.Transition(ctx, refund.ID, service.RefundChange{
			From: service.RefundPending, To: service.RefundRejected, ReviewNote: &note, ReviewedAt: &now,
		}); err != nil {
			return nil, err
		}
		if refund, err = s.find(ctx, refundNo); err != nil {
			return nil, err
		}
	}
	if refund.Status != service.RefundRejected {
		return nil, service.ErrFailedPrecondition // approved first
	}
	return refund, nil
}

func (s *sRefund) Retry(ctx context.Context) error {
	due, err := s.refunds.DueApproved(ctx, 100)
	if err != nil {
		return err
	}
	for _, refund := range due {
		// One trace per request paid: the scan itself runs quiet.
		rctx, span := telemetry.Root(ctx, "order.after_sale_refund",
			attribute.String("refund.no", refund.No), attribute.Int("refund.attempt", refund.Attempts+1))
		if err := s.pay(rctx, refund); err != nil {
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
	return nil
}

// pay asks the provider to return an approved request's amount and records
// the answer. No answer (an error) leaves it APPROVED for the next Retry.
func (s *sRefund) pay(ctx context.Context, refund *service.Refund) error {
	refundID := service.AfterSaleRefundID(refund.No)
	if err := s.provider.Refund(ctx, refundID, service.PaymentID(refund.OrderNo), refund.AmountCents); err != nil {
		g.Log().Warningf(ctx, "after-sale %s: refund %s (attempt %d) failed, will retry: %v",
			refund.No, refundID, refund.Attempts+1, err)
		if err := s.refunds.Attempted(ctx, refund.ID); err != nil {
			g.Log().Warningf(ctx, "after-sale %s: record attempt: %v", refund.No, err)
		}
		return err
	}
	now := s.now()
	if _, err := s.refunds.Transition(ctx, refund.ID, service.RefundChange{
		From: service.RefundApproved, To: service.RefundRefunded, RefundedAt: &now,
	}); err != nil {
		g.Log().Warningf(ctx, "after-sale %s: record refunded, will retry: %v", refund.No, err)
		return err
	}
	g.Log().Infof(ctx, "after-sale %s: %d cents returned for order %s", refund.No, refund.AmountCents, refund.OrderNo)
	return nil
}

func (s *sRefund) find(ctx context.Context, refundNo string) (*service.Refund, error) {
	refund, err := s.refunds.FindByNo(ctx, refundNo)
	if err != nil {
		return nil, err
	}
	if refund == nil {
		return nil, service.ErrNotFound
	}
	return refund, nil
}

func newRefundNo(now time.Time) string {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("R%s%08d", now.UTC().Format("20060102150405"), n.Int64())
}
