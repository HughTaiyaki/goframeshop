// Package service defines the contracts of the order service.
package service

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	// ErrIdempotencyConflict: a request id was reused for a different order.
	ErrIdempotencyConflict = errors.New("request id reused with different parameters")
	// ErrFailedPrecondition: the order is in a state that does not allow it.
	ErrFailedPrecondition = errors.New("not allowed in the current state")
	// ErrGoodsUnavailable: a goods in the request does not exist or is off shelf.
	ErrGoodsUnavailable = errors.New("some goods are unavailable")
	// ErrBadNotice: a payment notification whose signature does not verify
	// or that is too old to accept.
	ErrBadNotice = errors.New("payment notification rejected")
	// ErrPaymentMismatch: the provider reports a payment that does not match
	// the order (amount, currency or order number). Needs a person.
	ErrPaymentMismatch = errors.New("payment does not match the order")
)

// Order states (docs/GoFrame微服务商城详细设计.md §6.1). SHIPPED and later
// are not implemented yet.
const (
	StatusReserving      = "RESERVING"
	StatusPendingPayment = "PENDING_PAYMENT"
	// StatusPaymentConfirming: the money arrived; the reserved stock is
	// being committed (sold).
	StatusPaymentConfirming = "PAYMENT_CONFIRMING"
	StatusPaid              = "PAID"
	// StatusException: paid, but the stock could not be committed. The
	// payment is refunded automatically, then the order closes.
	StatusException = "EXCEPTION"
	StatusClosing   = "CLOSING"
	StatusClosed    = "CLOSED"
)

// Why an order closed.
const (
	ReasonUserCancelled     = "USER_CANCELLED"
	ReasonPaymentTimeout    = "PAYMENT_TIMEOUT"
	ReasonReserveTimeout    = "RESERVE_TIMEOUT"
	ReasonInsufficientStock = "INSUFFICIENT_STOCK"
	ReasonGoodsUnavailable  = "GOODS_UNAVAILABLE"
	ReasonReserveConflict   = "RESERVE_CONFLICT"
	// ReasonInventoryConflict: paid, but the stock could not be committed.
	ReasonInventoryConflict = "INVENTORY_CONFLICT"
)

// Payment states, the same names on both sides (order service, simulator).
const (
	PaymentCreated   = "CREATED"
	PaymentSucceeded = "SUCCEEDED"
	PaymentClosed    = "CLOSED"
	PaymentRefunded  = "REFUNDED"
)

// Currency: everything is priced in CNY cents.
const Currency = "CNY"

// Technical refunds: money the shop gives back on its own, without an
// after-sale request.
const (
	TechRefundPending   = "PENDING"
	TechRefundSucceeded = "SUCCEEDED"
	// RefundLatePayment: paid after the order had started closing.
	RefundLatePayment = "LATE_PAYMENT"
	// RefundCommitFailed: paid, but the stock could not be committed.
	RefundCommitFailed = "COMMIT_FAILED"
)

// What recording a confirmed payment did (PaymentRepository.ConfirmPaid).
const (
	// PaidConfirming: the order was waiting for it; now PAYMENT_CONFIRMING.
	PaidConfirming = "CONFIRMING"
	// PaidDuplicate: already recorded (a repeated notification or query), or
	// the order is closing and its refund is already scheduled.
	PaidDuplicate = "DUPLICATE"
	// PaidLate: the order is closing or closed; a refund was scheduled.
	PaidLate = "LATE"
)

// Inventory outcomes, as the goods service names them.
const (
	OutcomeSucceeded         = "SUCCEEDED"
	OutcomeInsufficientStock = "INSUFFICIENT_STOCK"
	OutcomeGoodsUnavailable  = "GOODS_UNAVAILABLE"
	OutcomeAlreadyReleased   = "ALREADY_RELEASED"
	OutcomeStateConflict     = "STATE_CONFLICT"
	OutcomeNotReady          = "NOT_READY"
)

// Refund states.
const (
	RefundPending  = "PENDING"
	RefundApproved = "APPROVED"
	RefundRejected = "REJECTED"
)

// Limits.
const (
	MaxOrderLines     = 20
	MaxLineQuantity   = 99
	MaxRequestIDBytes = 64
	MaxRemarkRunes    = 255
	MaxReasonRunes    = 255
)

// EventPaymentPending is published when an order becomes payable, delayed
// until it expires (see rabbitmq.KeyOrderPaymentPending).
const EventPaymentPending = "order.payment_pending"

// PaymentPending is its payload.
type PaymentPending struct {
	OrderNo   string    `json:"orderNo"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ExpiryGrace delays the expiry message a little past expiresAt, so rounding
// of the message TTL can never deliver it a moment too early.
const ExpiryGrace = time.Second

// Timing settings, from config (order.*).
type Timing struct {
	// ReserveTimeout bounds RESERVING: past it the order closes and the
	// stock is released (design: 60s).
	ReserveTimeout time.Duration
	// PaymentTimeout is how long stock is held for an unpaid order
	// (design: 15m).
	PaymentTimeout time.Duration
}

type Order struct {
	ID               uint64
	No               string
	UserID           uint64
	RequestID        string
	RequestHash      string
	Status           string
	CloseReason      string
	TotalCents       uint64
	ConsigneeName    string
	ConsigneePhone   string
	ConsigneeAddress string
	Remark           string
	FromCart         bool
	ReserveDeadline  time.Time
	ExpiresAt        *time.Time
	PaidAt           *time.Time
	ClosedAt         *time.Time
	CreatedAt        time.Time
	Items            []OrderItem
}

// PaymentID is the order's payment at the provider. It is derived from the
// order number, so every retry, the expiry worker and the notification all
// name the same payment.
func PaymentID(orderNo string) string { return "P" + orderNo }

// TechRefundID is fixed per payment, so a technical refund happens at most
// once however often it is retried.
func TechRefundID(paymentID string) string { return "TR" + paymentID }

// Payment is the order service's own record of an order's payment.
type Payment struct {
	ID           string
	OrderID      uint64
	AmountCents  uint64
	State        string
	ProviderTxID string
}

type TechRefund struct {
	ID          string
	PaymentID   string
	OrderID     uint64
	AmountCents uint64
	Reason      string
	State       string
	Attempts    int
}

// PaymentView is what the shopper gets back from Pay: where to pay, and the
// order as it is now.
type PaymentView struct {
	PaymentID   string
	State       string
	AmountCents uint64
	PayURL      string
	ExpiresAt   *time.Time
	Order       *Order
}

type OrderItem struct {
	GoodsID        uint64
	GoodsName      string
	CoverURL       string
	UnitPriceCents uint64
	Quantity       uint32
	SubtotalCents  uint64
}

type ItemInput struct {
	GoodsID  uint64
	Quantity uint32
}

type CreateInput struct {
	RequestID   string
	Items       []ItemInput
	ConsigneeID uint64
	Remark      string
	// FromCart is copied into order.placed; it does not change the order.
	FromCart bool
}

// ListFilter selects orders. UserID 0 means everyone (administrators only).
type ListFilter struct {
	UserID      uint64
	OrderNo     string
	Status      string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Page, Size  int
}

type Refund struct {
	ID        uint64
	No        string
	OrderID   uint64
	OrderNo   string
	UserID    uint64
	GoodsID   uint64
	Reason    string
	Status    string
	CreatedAt time.Time
}

// IOrder is the order application service.
type IOrder interface {
	Create(ctx context.Context, userID uint64, in *CreateInput) (*Order, error)
	Cancel(ctx context.Context, userID uint64, orderNo string) (*Order, error)
	GetMine(ctx context.Context, userID uint64, orderNo string) (*Order, error)
	ListMine(ctx context.Context, userID uint64, status string, page, size int) ([]*Order, int, error)
	Get(ctx context.Context, orderNo string) (*Order, error)
	List(ctx context.Context, f *ListFilter) ([]*Order, int, error)
	// Recover drives every order that is waiting on something: RESERVING
	// orders get their reservation retried (or closed past the deadline),
	// expired PENDING_PAYMENT orders close, CLOSING orders get their
	// payment closed and stock released, PAYMENT_CONFIRMING orders get their
	// stock committed, and pending technical refunds are retried. The
	// background worker calls it on a timer.
	Recover(ctx context.Context) error
	// Settle drives one order the way Recover drives each due one. The
	// expiry consumer calls it right after closing an order, so the payment
	// is closed and the stock released without waiting for the next scan.
	Settle(ctx context.Context, orderNo string) error
	// Pay opens (or reopens) the payment of the shopper's own order and
	// returns where to pay. If the provider says it is already paid, the
	// payment is confirmed on the spot: this is also how a shopper recovers
	// from a lost notification.
	Pay(ctx context.Context, userID uint64, orderNo string) (*PaymentView, error)
	// Notify handles the provider's signed notification. It only trusts the
	// signature to decide whether to look: the facts come from querying the
	// provider. nil means "acknowledged".
	Notify(ctx context.Context, body []byte, signature string) error
}

// IRefund is the after-sale application service.
type IRefund interface {
	Create(ctx context.Context, userID uint64, orderNo string, goodsID uint64, reason string) (*Refund, error)
	GetMine(ctx context.Context, userID uint64, refundNo string) (*Refund, error)
	ListMine(ctx context.Context, userID uint64, page, size int) ([]*Refund, int, error)
	List(ctx context.Context, status string, page, size int) ([]*Refund, int, error)
}
