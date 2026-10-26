package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

type paymentRepository struct{}

// NewPayment returns the MySQL-backed payment repository.
func NewPayment() service.PaymentRepository {
	return &paymentRepository{}
}

func (r *paymentRepository) Create(ctx context.Context, p *service.Payment) error {
	_, err := dao.Payment.Ctx(ctx).Data(do.Payment{
		PaymentId: p.ID, OrderId: p.OrderID, AmountCents: p.AmountCents, State: p.State,
	}).InsertIgnore()
	return err
}

func (r *paymentRepository) Find(ctx context.Context, paymentID string) (*service.Payment, error) {
	var row *entity.Payment
	err := dao.Payment.Ctx(ctx).Where(dao.Payment.Columns().PaymentId, paymentID).Scan(&row)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &service.Payment{
		ID: row.PaymentId, OrderID: row.OrderId, AmountCents: row.AmountCents,
		State: row.State, ProviderTxID: row.ProviderTxId,
	}, nil
}

func (r *paymentRepository) MarkClosed(ctx context.Context, paymentID string) error {
	columns := dao.Payment.Columns()
	_, err := dao.Payment.Ctx(ctx).Data(columns.State, service.PaymentClosed).
		Where(columns.PaymentId, paymentID).
		Where(columns.State, service.PaymentCreated).
		Update()
	return err
}

// ConfirmPaid locks the order row first. Every other change to the order
// (Transition) is an UPDATE of the same row and waits for this transaction,
// so "payment arrived" and "order closing" can never both win.
func (r *paymentRepository) ConfirmPaid(ctx context.Context, orderID uint64, p *service.Payment, paidAt time.Time) (string, error) {
	var outcome string
	err := dao.OrderInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		orderColumns := dao.OrderInfo.Columns()
		var order *entity.OrderInfo
		err := dao.OrderInfo.Ctx(ctx).TX(tx).Where(orderColumns.Id, orderID).LockUpdate().Scan(&order)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if order == nil {
			return service.ErrNotFound
		}

		// The fact: the payment succeeded. Never downgrades a REFUNDED one.
		paymentColumns := dao.Payment.Columns()
		if _, err := dao.Payment.Ctx(ctx).TX(tx).Data(do.Payment{
			PaymentId: p.ID, OrderId: orderID, AmountCents: p.AmountCents, State: service.PaymentCreated,
		}).InsertIgnore(); err != nil {
			return err
		}
		if _, err := dao.Payment.Ctx(ctx).TX(tx).Data(gdb.Map{
			paymentColumns.State:        service.PaymentSucceeded,
			paymentColumns.ProviderTxId: p.ProviderTxID,
		}).Where(paymentColumns.PaymentId, p.ID).
			WhereIn(paymentColumns.State, []string{service.PaymentCreated, service.PaymentClosed}).
			Update(); err != nil {
			return err
		}

		switch order.Status {
		case service.StatusPendingPayment:
			outcome = service.PaidConfirming
			_, err := dao.OrderInfo.Ctx(ctx).TX(tx).Data(gdb.Map{
				orderColumns.Status: service.StatusPaymentConfirming,
				orderColumns.PaidAt: gtime.New(paidAt),
			}).Where(orderColumns.Id, orderID).Update()
			return err
		case service.StatusPaymentConfirming, service.StatusPaid, service.StatusException:
			outcome = service.PaidDuplicate
			return nil
		default:
			// Closing or closed: keep it that way and give the money back.
			// A refund that already exists means we have been here before.
			scheduled, err := scheduleRefund(ctx, tx, p.ID, orderID, p.AmountCents, service.RefundLatePayment)
			outcome = service.PaidDuplicate
			if scheduled {
				outcome = service.PaidLate
			}
			return err
		}
	})
	return outcome, err
}

func (r *paymentRepository) Except(ctx context.Context, orderID uint64, paymentID string) (bool, error) {
	var changed bool
	err := dao.OrderInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		orderColumns := dao.OrderInfo.Columns()
		result, err := dao.OrderInfo.Ctx(ctx).TX(tx).Data(gdb.Map{
			orderColumns.Status:      service.StatusException,
			orderColumns.CloseReason: service.ReasonInventoryConflict,
		}).Where(orderColumns.Id, orderID).
			Where(orderColumns.Status, service.StatusPaymentConfirming).
			Update()
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return err
		}
		changed = true
		var payment *entity.Payment
		if err := dao.Payment.Ctx(ctx).TX(tx).Where(dao.Payment.Columns().PaymentId, paymentID).Scan(&payment); err != nil {
			return err // includes sql.ErrNoRows: a confirming order always has its payment
		}
		_, err = scheduleRefund(ctx, tx, paymentID, orderID, payment.AmountCents, service.RefundCommitFailed)
		return err
	})
	return changed, err
}

// scheduleRefund inserts the payment's one technical refund, unless it has
// one already (UNIQUE payment_id). It reports whether it inserted one.
func scheduleRefund(ctx context.Context, tx gdb.TX, paymentID string, orderID, amount uint64, reason string) (bool, error) {
	result, err := dao.TechRefund.Ctx(ctx).TX(tx).Data(do.TechRefund{
		RefundId:    service.TechRefundID(paymentID),
		PaymentId:   paymentID,
		OrderId:     orderID,
		AmountCents: amount,
		Reason:      reason,
		State:       service.TechRefundPending,
	}).InsertIgnore()
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *paymentRepository) DueRefunds(ctx context.Context, limit int) ([]*service.TechRefund, error) {
	columns := dao.TechRefund.Columns()
	var rows []*entity.TechRefund
	err := dao.TechRefund.Ctx(ctx).Where(columns.State, service.TechRefundPending).
		OrderAsc(columns.CreatedAt).Limit(limit).Scan(&rows)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	out := make([]*service.TechRefund, 0, len(rows))
	for _, row := range rows {
		out = append(out, &service.TechRefund{
			ID: row.RefundId, PaymentID: row.PaymentId, OrderID: row.OrderId, AmountCents: row.AmountCents,
			Reason: row.Reason, State: row.State, Attempts: int(row.Attempts),
		})
	}
	return out, nil
}

func (r *paymentRepository) Refunded(ctx context.Context, refundID string) (bool, error) {
	var closing bool
	err := dao.OrderInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		columns := dao.TechRefund.Columns()
		var refund *entity.TechRefund
		err := dao.TechRefund.Ctx(ctx).TX(tx).Where(columns.RefundId, refundID).LockUpdate().Scan(&refund)
		if err != nil || refund.State != service.TechRefundPending {
			return err
		}
		if _, err := dao.TechRefund.Ctx(ctx).TX(tx).Data(columns.State, service.TechRefundSucceeded).
			Where(columns.RefundId, refundID).Update(); err != nil {
			return err
		}
		paymentColumns := dao.Payment.Columns()
		if _, err := dao.Payment.Ctx(ctx).TX(tx).Data(paymentColumns.State, service.PaymentRefunded).
			Where(paymentColumns.PaymentId, refund.PaymentId).Update(); err != nil {
			return err
		}
		// An EXCEPTION order waited for this refund; now it can close.
		orderColumns := dao.OrderInfo.Columns()
		result, err := dao.OrderInfo.Ctx(ctx).TX(tx).Data(orderColumns.Status, service.StatusClosing).
			Where(orderColumns.Id, refund.OrderId).
			Where(orderColumns.Status, service.StatusException).
			Update()
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		closing = n == 1
		return err
	})
	return closing, err
}

func (r *paymentRepository) RefundAttempted(ctx context.Context, refundID string) error {
	columns := dao.TechRefund.Columns()
	_, err := dao.TechRefund.Ctx(ctx).Data(columns.Attempts, gdb.Raw(columns.Attempts+" + 1")).
		Where(columns.RefundId, refundID).Update()
	return err
}
