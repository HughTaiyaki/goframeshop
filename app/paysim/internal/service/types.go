// Package service defines the contracts of the payment simulator.
package service

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
	// ErrConflict: the payment or refund id already names something else.
	ErrConflict = errors.New("id already used with different parameters")
	// ErrFailedPrecondition: not allowed in the payment's current state.
	ErrFailedPrecondition = errors.New("not allowed in the current state")
	// ErrExpired: the payer is too late; the payment can no longer be paid.
	ErrExpired = errors.New("payment expired")
)

// Payment states.
const (
	StateCreated   = "CREATED"
	StateSucceeded = "SUCCEEDED"
	StateClosed    = "CLOSED"
	StateRefunded  = "REFUNDED"
)

// Currency is the only one the simulator accepts.
const Currency = "CNY"

type Payment struct {
	ID          string
	OrderNo     string
	AmountCents uint64 // 0 for a tombstone left by Close
	Currency    string
	State       string
	ExpiresAt   *time.Time
	TxID        string
	PaidAt      *time.Time
	// Notification bookkeeping. NextNotifyAt nil: nothing to send.
	NotifyAttempts int
	NextNotifyAt   *time.Time
}

// Paid reports whether the payer's money arrived (refunded or not).
func (p *Payment) Paid() bool {
	return p.State == StateSucceeded || p.State == StateRefunded
}

type Refund struct {
	ID          string
	PaymentID   string
	AmountCents uint64
}

type CreateInput struct {
	PaymentID   string
	OrderNo     string
	AmountCents uint64
	Currency    string
	ExpiresAt   time.Time
}

// Write is what a decision changes: the payment to insert or update, and a
// refund to insert. Nil fields are left alone.
type Write struct {
	Payment *Payment
	Refund  *Refund
}

// Decide looks at a payment (nil when it does not exist) and its refunds
// (oldest first; several partial ones since task 0018) and says what to
// write: at most one new refund per decision.
type Decide func(current *Payment, refunds []*Refund) (Write, error)

// Repository persists payments. Update is the only way to change one: it
// locks the payment row, calls decide and applies its Write in the same
// transaction, so concurrent Pay and Close are decided one after the other.
type Repository interface {
	Update(ctx context.Context, paymentID string, decide Decide) (*Payment, error)
	Find(ctx context.Context, paymentID string) (*Payment, error)
	// DueNotifications returns paid payments whose notification is due.
	DueNotifications(ctx context.Context, now time.Time, limit int) ([]*Payment, error)
	// Notified records one delivery attempt. next nil: stop notifying.
	Notified(ctx context.Context, paymentID string, attempts int, next *time.Time) error
}

// IProvider is the simulator's merchant API plus the payer's checkout action.
type IProvider interface {
	Create(ctx context.Context, in *CreateInput) (*Payment, error)
	Close(ctx context.Context, paymentID, orderNo string) (*Payment, error)
	Query(ctx context.Context, paymentID string) (*Payment, error)
	Refund(ctx context.Context, refundID, paymentID string, amountCents uint64) (*Refund, error)
	// Pay is the payer paying on the checkout page. notify false pays
	// without ever notifying the merchant, to simulate a lost notification.
	Pay(ctx context.Context, paymentID string, notify bool) (*Payment, error)
}
