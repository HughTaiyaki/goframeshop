// Package provider decides what happens to a payment. Every decision runs
// inside Repository.Update on the locked payment row, so two requests for
// the same payment (the payer paying, the merchant closing) are decided one
// after the other and see each other's result:
//
//	            Pay                 Refund
//	CREATED ──────────▶ SUCCEEDED ──────────▶ REFUNDED
//	   │ Close
//	   ▼
//	 CLOSED   (also written as a tombstone when Close comes before Create,
//	           so the payment can never be created or paid afterwards)
//
// All calls are idempotent: repeating one returns the payment as it is.
package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

type sProvider struct {
	payments service.Repository
	now      func() time.Time
}

func New(payments service.Repository, now func() time.Time) service.IProvider {
	return &sProvider{payments: payments, now: now}
}

func (s *sProvider) Create(ctx context.Context, in *service.CreateInput) (*service.Payment, error) {
	if in.PaymentID == "" || in.OrderNo == "" || in.AmountCents == 0 || in.Currency != service.Currency || in.ExpiresAt.IsZero() {
		return nil, service.ErrInvalidArgument
	}
	return s.payments.Update(ctx, in.PaymentID, func(p *service.Payment, _ *service.Refund) (service.Write, error) {
		if p == nil {
			expires := in.ExpiresAt
			return service.Write{Payment: &service.Payment{
				ID: in.PaymentID, OrderNo: in.OrderNo, AmountCents: in.AmountCents,
				Currency: in.Currency, State: service.StateCreated, ExpiresAt: &expires,
			}}, nil
		}
		if p.OrderNo != in.OrderNo {
			return service.Write{}, service.ErrConflict
		}
		if isTombstone(p) {
			return service.Write{}, nil // closed before it was created: stays closed
		}
		if p.AmountCents != in.AmountCents || p.Currency != in.Currency {
			return service.Write{}, service.ErrConflict
		}
		return service.Write{}, nil // a retry: answer with the payment as it is
	})
}

func (s *sProvider) Close(ctx context.Context, paymentID, orderNo string) (*service.Payment, error) {
	if paymentID == "" || orderNo == "" {
		return nil, service.ErrInvalidArgument
	}
	return s.payments.Update(ctx, paymentID, func(p *service.Payment, _ *service.Refund) (service.Write, error) {
		if p == nil {
			// The tombstone: a Create or Pay arriving later finds it closed.
			return service.Write{Payment: &service.Payment{
				ID: paymentID, OrderNo: orderNo, Currency: service.Currency, State: service.StateClosed,
			}}, nil
		}
		if p.OrderNo != orderNo {
			return service.Write{}, service.ErrConflict
		}
		if p.State != service.StateCreated {
			// Already closed, or paid: a paid payment is never closed. The
			// merchant reads the state from the answer and refunds instead.
			return service.Write{}, nil
		}
		closed := *p
		closed.State = service.StateClosed
		return service.Write{Payment: &closed}, nil
	})
}

func (s *sProvider) Query(ctx context.Context, paymentID string) (*service.Payment, error) {
	if paymentID == "" {
		return nil, service.ErrInvalidArgument
	}
	p, err := s.payments.Find(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, service.ErrNotFound
	}
	return p, nil
}

func (s *sProvider) Refund(ctx context.Context, refundID, paymentID string, amountCents uint64) (*service.Refund, error) {
	if refundID == "" || paymentID == "" || amountCents == 0 {
		return nil, service.ErrInvalidArgument
	}
	var result *service.Refund
	_, err := s.payments.Update(ctx, paymentID, func(p *service.Payment, r *service.Refund) (service.Write, error) {
		if p == nil {
			return service.Write{}, service.ErrNotFound
		}
		if r != nil {
			// Only full refunds, so a payment has at most one refund.
			if r.ID != refundID || r.AmountCents != amountCents {
				return service.Write{}, service.ErrConflict
			}
			result = r
			return service.Write{}, nil // the same refund again: done once
		}
		if p.State != service.StateSucceeded {
			return service.Write{}, service.ErrFailedPrecondition
		}
		if amountCents != p.AmountCents {
			return service.Write{}, service.ErrInvalidArgument
		}
		refunded := *p
		refunded.State = service.StateRefunded
		result = &service.Refund{ID: refundID, PaymentID: paymentID, AmountCents: amountCents}
		return service.Write{Payment: &refunded, Refund: result}, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *sProvider) Pay(ctx context.Context, paymentID string, notify bool) (*service.Payment, error) {
	if paymentID == "" {
		return nil, service.ErrInvalidArgument
	}
	return s.payments.Update(ctx, paymentID, func(p *service.Payment, _ *service.Refund) (service.Write, error) {
		if p == nil {
			return service.Write{}, service.ErrNotFound
		}
		switch p.State {
		case service.StateSucceeded, service.StateRefunded:
			return service.Write{}, nil // paying twice charges once
		case service.StateClosed:
			return service.Write{}, service.ErrFailedPrecondition
		}
		now := s.now()
		if p.ExpiresAt != nil && !now.Before(*p.ExpiresAt) {
			return service.Write{}, service.ErrExpired
		}
		paid := *p
		paid.State = service.StateSucceeded
		paid.TxID = newTxID(now)
		paid.PaidAt = &now
		paid.NotifyAttempts = 0
		paid.NextNotifyAt = nil
		if notify {
			paid.NextNotifyAt = &now
		}
		return service.Write{Payment: &paid}, nil
	})
}

func isTombstone(p *service.Payment) bool {
	return p.State == service.StateClosed && p.AmountCents == 0
}

// newTxID is the provider's transaction number: "T" + UTC time + 10 random
// digits, the kind of id a real provider returns.
func newTxID(now time.Time) string {
	n, err := rand.Int(rand.Reader, big.NewInt(10_000_000_000))
	if err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return fmt.Sprintf("T%s%010d", now.UTC().Format("20060102150405"), n.Int64())
}
