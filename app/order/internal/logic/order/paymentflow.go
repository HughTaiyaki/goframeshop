package order

// The payment half of the order (design §6.1, §6.4).
//
//	shopper ──Pay──▶ order ──Create──▶ provider ──pay_url──▶ shopper pays on the checkout page
//	provider ──signed notification──▶ gateway-h5 ──Notify──▶ order ──Query──▶ provider
//	                                                          └─ confirm ─▶ PAYMENT_CONFIRMING ─Commit─▶ PAID
//
// Three rules keep money and stock straight:
//
//   - The notification is only a hint. Its signature decides whether to look;
//     what happened is read from the provider (Query), checked against the
//     order (number, amount, currency), and only then recorded.
//   - Recording a payment and moving the order happen in one transaction on
//     the locked order row (PaymentRepository.ConfirmPaid). If the order is
//     still waiting, it goes on to PAYMENT_CONFIRMING; if it already started
//     closing, the payment is refunded instead and the order stays closed.
//   - Every outside call can be repeated: the payment id, the commit
//     operation id and the refund id are all derived from the order number.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// noticeMaxAge bounds how old a notification may be. Each delivery attempt
// is signed afresh, so this only refuses replays of old captured ones.
const noticeMaxAge = 5 * time.Minute

func (s *sOrder) Pay(ctx context.Context, userID uint64, orderNo string) (*service.PaymentView, error) {
	order, err := s.GetMine(ctx, userID, orderNo)
	if err != nil {
		return nil, err
	}
	switch order.Status {
	case service.StatusPendingPayment:
		if order.ExpiresAt == nil || !s.now().Before(*order.ExpiresAt) {
			return nil, service.ErrFailedPrecondition // expired; the worker is about to close it
		}
	case service.StatusPaymentConfirming, service.StatusPaid:
		// Already paid: answer with the payment as it stands.
	default:
		return nil, service.ErrFailedPrecondition
	}

	paymentID := service.PaymentID(order.No)
	// Our own record first, so the payment is never unknown to us.
	if err := s.payments.Create(ctx, &service.Payment{
		ID: paymentID, OrderID: order.ID, AmountCents: order.TotalCents, State: service.PaymentCreated,
	}); err != nil {
		return nil, err
	}
	p, err := s.provider.Create(ctx, paymentID, order.No, order.TotalCents, *order.ExpiresAt)
	if err != nil {
		return nil, err
	}
	switch {
	case p.Paid():
		// Paid, and we may not have been told: confirm now.
		if err := s.confirm(ctx, order, p); err != nil {
			return nil, err
		}
		if order, err = s.orders.FindByNo(ctx, order.No); err != nil {
			return nil, err
		}
	case p.State == service.PaymentClosed:
		return nil, service.ErrFailedPrecondition // the order is being closed
	}
	return &service.PaymentView{
		PaymentID: p.ID, State: p.State, AmountCents: p.AmountCents,
		PayURL: payURL(p), ExpiresAt: p.ExpiresAt, Order: order,
	}, nil
}

func payURL(p *service.ProviderPayment) string {
	if p.State != service.PaymentCreated {
		return "" // nothing left to pay
	}
	return p.PayURL
}

func (s *sOrder) Notify(ctx context.Context, body []byte, signature string) error {
	if !s.notices.Verify(body, signature) {
		g.Log().Warningf(ctx, "payment notification with a bad signature (%d bytes) refused", len(body))
		return service.ErrBadNotice
	}
	var notice utility.PaymentNotice
	if err := json.Unmarshal(body, &notice); err != nil || notice.PaymentID == "" {
		return service.ErrInvalidArgument
	}
	if age := s.now().Sub(notice.NotifyTime); age > noticeMaxAge || age < -noticeMaxAge {
		g.Log().Warningf(ctx, "payment %s: notification from %s is too old or from the future; refused",
			notice.PaymentID, notice.NotifyTime.Format(time.RFC3339))
		return service.ErrBadNotice
	}
	local, err := s.payments.Find(ctx, notice.PaymentID)
	if err != nil {
		return err
	}
	if local == nil {
		return service.ErrNotFound // not one of ours
	}
	// The facts come from the provider, not from the notification.
	p, err := s.provider.Query(ctx, notice.PaymentID)
	if err != nil {
		return err
	}
	if !p.Paid() {
		return nil // nothing to do; acknowledge
	}
	order, err := s.orders.FindByNo(ctx, p.OrderNo)
	if err != nil {
		return err
	}
	if order == nil || order.ID != local.OrderID {
		g.Log().Errorf(ctx, "payment %s: provider names order %s, which is not the order it was opened for", p.ID, p.OrderNo)
		return service.ErrPaymentMismatch
	}
	return s.confirm(ctx, order, p)
}

// confirm records a payment the provider says is paid, then acts on what
// ConfirmPaid decided. Safe to call any number of times.
func (s *sOrder) confirm(ctx context.Context, order *service.Order, p *service.ProviderPayment) error {
	if p.OrderNo != order.No || p.AmountCents != order.TotalCents || p.Currency != service.Currency || p.TxID == "" {
		g.Log().Errorf(ctx, "payment %s does not match order %s: provider says order %s, %d %s (order total %d %s); left for manual review",
			p.ID, order.No, p.OrderNo, p.AmountCents, p.Currency, order.TotalCents, service.Currency)
		return service.ErrPaymentMismatch
	}
	paidAt := s.now()
	if p.PaidAt != nil {
		paidAt = *p.PaidAt
	}
	outcome, err := s.payments.ConfirmPaid(ctx, order.ID, &service.Payment{
		ID: p.ID, OrderID: order.ID, AmountCents: p.AmountCents, State: service.PaymentSucceeded, ProviderTxID: p.TxID,
	}, paidAt)
	if err != nil {
		g.Log().Warningf(ctx, "order %s: record payment %s: %v", order.No, p.ID, err)
		return err
	}
	switch outcome {
	case service.PaidConfirming:
		g.Log().Infof(ctx, "order %s: payment %s confirmed (tx %s); committing stock", order.No, p.ID, p.TxID)
		s.commit(ctx, order)
	case service.PaidLate:
		g.Log().Warningf(ctx, "order %s: payment %s arrived after the order started closing; refund %s scheduled",
			order.No, p.ID, service.TechRefundID(p.ID))
	}
	return nil
}

// commit turns the reservation into a sale. With no answer the order stays
// PAYMENT_CONFIRMING and the worker tries again with the same operation id.
func (s *sOrder) commit(ctx context.Context, order *service.Order) {
	outcome, err := s.inventory.Commit(ctx, "commit-"+order.No, order.No)
	if err != nil {
		g.Log().Warningf(ctx, "order %s: commit outcome unknown, the worker will retry: %v", order.No, err)
		return
	}
	switch outcome {
	case service.OutcomeSucceeded:
		if s.transition(ctx, order, service.Change{From: []string{service.StatusPaymentConfirming}, To: service.StatusPaid}) {
			g.Log().Infof(ctx, "order %s: paid", order.No)
		}
	case service.OutcomeNotReady:
		g.Log().Warningf(ctx, "order %s: commit not ready, the worker will retry", order.No)
	default:
		// The reservation is gone (released). The money must go back.
		g.Log().Errorf(ctx, "order %s: commit refused with %s; refunding and closing", order.No, outcome)
		if _, err := s.payments.Except(ctx, order.ID, service.PaymentID(order.No)); err != nil {
			g.Log().Warningf(ctx, "order %s: move to EXCEPTION failed, the worker will retry: %v", order.No, err)
		}
	}
}

// refundPending retries every technical refund that is not done yet.
func (s *sOrder) refundPending(ctx context.Context) error {
	refunds, err := s.payments.DueRefunds(ctx, 100)
	if err != nil {
		return err
	}
	for _, r := range refunds {
		s.refundOne(ctx, r)
	}
	return nil
}

func (s *sOrder) refundOne(ctx context.Context, r *service.TechRefund) {
	ctx, span := telemetry.Root(ctx, "order.tech_refund",
		attribute.String("refund.id", r.ID), attribute.String("refund.reason", r.Reason), attribute.Int("refund.attempt", r.Attempts+1))
	defer span.End()
	if err := s.provider.Refund(ctx, r.ID, r.PaymentID, r.AmountCents); err != nil {
		g.Log().Warningf(ctx, "refund %s (%s, payment %s, attempt %d) failed, will retry: %v",
			r.ID, r.Reason, r.PaymentID, r.Attempts+1, err)
		span.SetStatus(codes.Error, err.Error())
		if err := s.payments.RefundAttempted(ctx, r.ID); err != nil {
			g.Log().Warningf(ctx, "refund %s: record attempt: %v", r.ID, err)
		}
		return
	}
	if err := s.payments.Refunded(ctx, r.ID); err != nil {
		g.Log().Warningf(ctx, "refund %s: record success, will retry: %v", r.ID, err)
		return
	}
	g.Log().Infof(ctx, "refund %s (%s): %d cents returned for payment %s", r.ID, r.Reason, r.AmountCents, r.PaymentID)
}
