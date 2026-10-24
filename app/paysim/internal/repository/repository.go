// Package repository stores the simulator's payments in its own database.
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// insertRetries: two first requests for the same payment both find no row
// and both insert; the loser retries and then sees the winner's row.
const insertRetries = 3

type repository struct{}

func New() service.Repository {
	return &repository{}
}

// Update locks the payment row (SELECT ... FOR UPDATE), lets decide look at
// it and its refunds, and writes the decision before committing. READ
// COMMITTED avoids gap locks on rows that do not exist yet.
func (r *repository) Update(ctx context.Context, paymentID string, decide service.Decide) (*service.Payment, error) {
	var (
		result *service.Payment
		err    error
	)
	for attempt := 0; attempt < insertRetries; attempt++ {
		err = dao.Payment.DB().TransactionWithOptions(ctx,
			gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelReadCommitted},
			func(ctx context.Context, tx gdb.TX) error {
				current, err := findPayment(dao.Payment.Ctx(ctx).TX(tx).LockUpdate(), paymentID)
				if err != nil {
					return err
				}
				refunds, err := findRefunds(ctx, tx, paymentID)
				if err != nil {
					return err
				}
				write, err := decide(current, refunds)
				if err != nil {
					return err
				}
				result = current
				if write.Payment != nil {
					if err := savePayment(ctx, tx, current == nil, write.Payment); err != nil {
						return err
					}
					result = write.Payment
				}
				if write.Refund != nil {
					_, err := dao.Refund.Ctx(ctx).TX(tx).Data(do.Refund{
						RefundId: write.Refund.ID, PaymentId: write.Refund.PaymentID, AmountCents: write.Refund.AmountCents,
					}).Insert()
					return err
				}
				return nil
			})
		if !utility.IsDuplicateKey(err) {
			return result, err
		}
	}
	// Still a duplicate after retries: the refund id belongs to another payment.
	return nil, service.ErrConflict
}

func savePayment(ctx context.Context, tx gdb.TX, insert bool, p *service.Payment) error {
	// A map rather than do.Payment: nils here are meant (clear the column),
	// and do structs skip nil fields.
	columns := dao.Payment.Columns()
	data := gdb.Map{
		columns.State:          p.State,
		columns.ProviderTxId:   nilIfEmpty(p.TxID),
		columns.PaidAt:         timeOrNil(p.PaidAt),
		columns.NotifyAttempts: p.NotifyAttempts,
		columns.NextNotifyAt:   timeOrNil(p.NextNotifyAt),
	}
	if !insert {
		_, err := dao.Payment.Ctx(ctx).TX(tx).Data(data).Where(columns.PaymentId, p.ID).Update()
		return err
	}
	data[columns.PaymentId] = p.ID
	data[columns.OrderNo] = p.OrderNo
	data[columns.AmountCents] = p.AmountCents
	data[columns.Currency] = p.Currency
	data[columns.ExpiresAt] = timeOrNil(p.ExpiresAt)
	_, err := dao.Payment.Ctx(ctx).TX(tx).Data(data).Insert()
	return err
}

func (r *repository) Find(ctx context.Context, paymentID string) (*service.Payment, error) {
	return findPayment(dao.Payment.Ctx(ctx), paymentID)
}

func (r *repository) DueNotifications(ctx context.Context, now time.Time, limit int) ([]*service.Payment, error) {
	columns := dao.Payment.Columns()
	all, err := dao.Payment.Ctx(ctx).
		WhereLTE(columns.NextNotifyAt, gtime.New(now)).
		OrderAsc(columns.NextNotifyAt).
		Limit(limit).
		All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.Payment
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	out := make([]*service.Payment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPayment(row))
	}
	return out, nil
}

func (r *repository) Notified(ctx context.Context, paymentID string, attempts int, next *time.Time) error {
	columns := dao.Payment.Columns()
	_, err := dao.Payment.Ctx(ctx).Data(gdb.Map{
		columns.NotifyAttempts: attempts,
		columns.NextNotifyAt:   timeOrNil(next),
	}).Where(columns.PaymentId, paymentID).Update()
	return err
}

func findPayment(model *gdb.Model, paymentID string) (*service.Payment, error) {
	var row *entity.Payment
	if err := model.Where(dao.Payment.Columns().PaymentId, paymentID).Scan(&row); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return toPayment(row), nil
}

// findRefunds reads the refunds of a payment whose row the caller has
// locked: a new refund for it can only be added under the same lock, so the
// list is complete for the decision.
func findRefunds(ctx context.Context, tx gdb.TX, paymentID string) ([]*service.Refund, error) {
	var rows []*entity.Refund
	err := dao.Refund.Ctx(ctx).TX(tx).
		Where(dao.Refund.Columns().PaymentId, paymentID).
		OrderAsc(dao.Refund.Columns().CreatedAt).
		Scan(&rows)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	refunds := make([]*service.Refund, 0, len(rows))
	for _, row := range rows {
		refunds = append(refunds, &service.Refund{ID: row.RefundId, PaymentID: row.PaymentId, AmountCents: row.AmountCents})
	}
	return refunds, nil
}

func toPayment(row *entity.Payment) *service.Payment {
	return &service.Payment{
		ID:             row.PaymentId,
		OrderNo:        row.OrderNo,
		AmountCents:    row.AmountCents,
		Currency:       row.Currency,
		State:          row.State,
		ExpiresAt:      timePtr(row.ExpiresAt),
		TxID:           row.ProviderTxId,
		PaidAt:         timePtr(row.PaidAt),
		NotifyAttempts: int(row.NotifyAttempts),
		NextNotifyAt:   timePtr(row.NextNotifyAt),
	}
}

func timePtr(t *gtime.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	v := t.Time
	return &v
}

func timeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return gtime.New(*t)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil // provider_tx_id is UNIQUE: unpaid rows must stay NULL
	}
	return s
}
