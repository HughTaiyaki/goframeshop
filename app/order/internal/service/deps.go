package service

import (
	"context"
	"time"

	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

// GoodsSnapshot is what the order copies from the goods service.
type GoodsSnapshot struct {
	Name       string
	PriceCents uint64
	CoverURL   string
}

// Catalog reads goods from the goods service. It returns ErrNotFound for
// goods that do not exist or are off shelf.
type Catalog interface {
	Goods(ctx context.Context, goodsID uint64) (*GoodsSnapshot, error)
}

// Address is the shipping address copied into an order.
type Address struct {
	Name    string
	Phone   string
	Address string
}

// Addresses reads the caller's own address from the user service, using the
// caller's token. ErrNotFound when it is not theirs or does not exist.
type Addresses interface {
	Consignee(ctx context.Context, consigneeID uint64) (*Address, error)
}

// Inventory calls the goods service's InventoryService with this service's
// own token. An error means the outcome is unknown (timeout, unavailable):
// the call may or may not have happened, and must be retried with the same
// operation id.
type Inventory interface {
	Reserve(ctx context.Context, operationID, orderNo string, items []ItemInput) (outcome string, err error)
	Commit(ctx context.Context, operationID, orderNo string) (outcome string, err error)
	Release(ctx context.Context, operationID, orderNo string, items []ItemInput) (outcome string, err error)
}

// ProviderPayment is a payment as the provider (the simulator) reports it.
type ProviderPayment struct {
	ID          string
	OrderNo     string
	AmountCents uint64
	Currency    string
	State       string
	TxID        string
	PayURL      string
	ExpiresAt   *time.Time
	PaidAt      *time.Time
}

// Paid reports whether the payer's money arrived (refunded since or not).
func (p *ProviderPayment) Paid() bool {
	return p.State == PaymentSucceeded || p.State == PaymentRefunded
}

// Provider calls the payment simulator with this service's own token. Every
// call is idempotent at the provider, so an error (unknown outcome) is
// handled by calling again later.
type Provider interface {
	Create(ctx context.Context, paymentID, orderNo string, amountCents uint64, expiresAt time.Time) (*ProviderPayment, error)
	// Close closes an unpaid payment, or leaves a tombstone if it does not
	// exist. A paid payment stays paid: read the state from the answer.
	Close(ctx context.Context, paymentID, orderNo string) (*ProviderPayment, error)
	// Query returns ErrNotFound for a payment the provider never saw.
	Query(ctx context.Context, paymentID string) (*ProviderPayment, error)
	Refund(ctx context.Context, refundID, paymentID string, amountCents uint64) error
}

// NoticeVerifier checks the provider's signature on a notification.
type NoticeVerifier interface {
	Verify(body []byte, signature string) bool
}

// Change is a conditional state change: it applies only while the order is
// still in one of From, so two actors (the shopper cancelling, the worker
// expiring) can never both act on the same order.
type Change struct {
	From        []string
	To          string
	CloseReason string
	ExpiresAt   *time.Time
	PaidAt      *time.Time
	ClosedAt    *time.Time
	// Shipment and ConfirmDeadline are written when shipping;
	// CompletedAt and CompleteReason when completing.
	Shipment        *Shipment
	ShippedAt       *time.Time
	ConfirmDeadline *time.Time
	CompletedAt     *time.Time
	CompleteReason  string
	// Events are written to the outbox in the same transaction, and only if
	// this change wins.
	Events []eventbus.Event
}

// OrderRepository is the persistence seam for orders.
type OrderRepository interface {
	// Create inserts the order and its items in one transaction and returns
	// the id. ErrAlreadyExists when (user, request id) is taken.
	Create(ctx context.Context, o *Order) (uint64, error)
	FindByNo(ctx context.Context, orderNo string) (*Order, error)
	FindByRequest(ctx context.Context, userID uint64, requestID string) (*Order, error)
	NosByID(ctx context.Context, ids []uint64) (map[uint64]string, error)
	List(ctx context.Context, f *ListFilter) ([]*Order, error)
	Count(ctx context.Context, f *ListFilter) (int, error)
	// Transition applies c and reports whether this call changed the row.
	Transition(ctx context.Context, orderID uint64, c Change) (bool, error)
	// Due returns orders needing the worker, oldest first: all RESERVING,
	// PAYMENT_CONFIRMING and CLOSING ones, PENDING_PAYMENT ones expired at
	// now, and SHIPPED ones past their confirm deadline.
	Due(ctx context.Context, now time.Time, limit int) ([]*Order, error)
}

// PaymentRepository records payments and technical refunds. The methods
// that also move the order do both in one transaction with the order row
// locked, so a payment and a close of the same order are decided one after
// the other (design §6.4).
type PaymentRepository interface {
	// Create records the payment about to be opened at the provider.
	// Creating it again is a no-op.
	Create(ctx context.Context, p *Payment) error
	Find(ctx context.Context, paymentID string) (*Payment, error)
	// MarkClosed records that the provider closed it unpaid.
	MarkClosed(ctx context.Context, paymentID string) error
	// ConfirmPaid records a payment the provider confirmed and, depending on
	// the order: PENDING_PAYMENT moves to PAYMENT_CONFIRMING (PaidConfirming);
	// already confirming, paid or in exception changes nothing
	// (PaidDuplicate); anything else gets a LATE_PAYMENT refund (PaidLate),
	// unless the payment has a refund already (PaidDuplicate).
	ConfirmPaid(ctx context.Context, orderID uint64, p *Payment, paidAt time.Time) (string, error)
	// Except moves a PAYMENT_CONFIRMING order to EXCEPTION and schedules a
	// COMMIT_FAILED refund. False if the order was not confirming.
	Except(ctx context.Context, orderID uint64, paymentID string) (bool, error)
	DueRefunds(ctx context.Context, limit int) ([]*TechRefund, error)
	// Refunded marks the refund done; an EXCEPTION order moves on to
	// CLOSING in the same transaction.
	// Refunded also moves an EXCEPTION order to CLOSING and reports whether
	// it did.
	Refunded(ctx context.Context, refundID string) (orderClosing bool, err error)
	RefundAttempted(ctx context.Context, refundID string) error
}

// RefundRepository is the persistence seam for refunds. Create returns
// ErrAlreadyExists when the (order, goods) pair already has a request.
type RefundRepository interface {
	Create(ctx context.Context, r *Refund) (uint64, error)
	FindByNo(ctx context.Context, refundNo string) (*Refund, error)
	List(ctx context.Context, userID uint64, status string, page, size int) ([]*Refund, error)
	Count(ctx context.Context, userID uint64, status string) (int, error)
	// ByOrder returns every request of one order.
	ByOrder(ctx context.Context, orderID uint64) ([]*Refund, error)
	// Transition applies c and reports whether this call changed the row.
	Transition(ctx context.Context, refundID uint64, c RefundChange) (bool, error)
	// DueApproved returns APPROVED requests, oldest first.
	DueApproved(ctx context.Context, limit int) ([]*Refund, error)
	// Attempted counts one failed refund call.
	Attempted(ctx context.Context, refundID uint64) error
}
