package service

import (
	"context"
	"errors"
)

// Operation kinds and outcomes, stored in inventory_operation.
const (
	KindReserve = "RESERVE"
	KindCommit  = "COMMIT"
	KindRelease = "RELEASE"

	OutcomeSucceeded         = "SUCCEEDED"
	OutcomeInsufficientStock = "INSUFFICIENT_STOCK"
	OutcomeGoodsUnavailable  = "GOODS_UNAVAILABLE"
	OutcomeAlreadyReleased   = "ALREADY_RELEASED"
	OutcomeStateConflict     = "STATE_CONFLICT"
	// OutcomeNotReady is returned but never stored, so the same operation id
	// can be retried once the reservation exists.
	OutcomeNotReady = "NOT_READY"
)

// Reservation states, stored in reservation.state.
const (
	StateReserved  = "RESERVED"
	StateCommitted = "COMMITTED"
	StateReleased  = "RELEASED"
)

// Request limits; the proto validates the same numbers.
const (
	MaxInventoryItems    = 50
	MaxInventoryQuantity = 10000
	MaxIDLength          = 64
)

// InventoryItem is one line of a Reserve or Release.
type InventoryItem struct {
	GoodsID  uint64
	Quantity uint32
}

// Reservation is one (order, goods) row.
type Reservation struct {
	ID       uint64
	OrderID  string
	GoodsID  uint64
	Quantity uint32
	State    string
}

// Operation is the idempotency record of one call.
type Operation struct {
	ID          string
	Kind        string
	OrderID     string
	RequestHash string
	Outcome     string
}

// OperationResult is what every inventory call returns: the immutable
// outcome, and the order's reservations as they are now.
type OperationResult struct {
	OperationID  string
	Outcome      string
	Reservations []Reservation
	// Replayed: the operation id was already recorded, so this is the stored
	// outcome of an earlier call, not a new decision.
	Replayed bool
}

// IInventory is the reservation application service.
type IInventory interface {
	Reserve(ctx context.Context, operationID, orderID string, items []InventoryItem) (*OperationResult, error)
	Commit(ctx context.Context, operationID, orderID string) (*OperationResult, error)
	Release(ctx context.Context, operationID, orderID string, items []InventoryItem) (*OperationResult, error)
	// Query returns (nil, nil) when the operation id is unknown.
	Query(ctx context.Context, operationID string) (*OperationResult, error)
}

// ErrOperationExists is returned by LedgerTx.InsertOperation when another
// transaction already recorded the same operation id.
var ErrOperationExists = errors.New("operation already recorded")

// Ledger runs reservation transactions. The rules live in the logic layer;
// the ledger only provides locked reads and writes inside one transaction.
type Ledger interface {
	// InTx runs fn in a single transaction and commits if fn returns nil. A
	// deadlock is retried a few times, so fn must not have side effects
	// outside the transaction.
	InTx(ctx context.Context, fn func(ctx context.Context, tx LedgerTx) error) error
	// FindOperation reads a committed operation outside any transaction.
	FindOperation(ctx context.Context, operationID string) (*Operation, error)
	// Reservations reads an order's reservations outside any transaction.
	Reservations(ctx context.Context, orderID string) ([]Reservation, error)
}

// LedgerTx is the set of locked operations available inside InTx.
//
// Transactions run at READ COMMITTED, so a locking read of a row that does
// not exist takes no gap lock. Callers lock the inventory rows of every goods
// they touch FIRST, in goods_id order: that serialises all work on one goods
// and gives every transaction the same lock order, so they cannot deadlock on
// each other.
type LedgerTx interface {
	FindOperation(operationID string) (*Operation, error)
	// InsertOperation is called first in every transaction: concurrent calls
	// with the same id then queue on its primary key instead of racing.
	InsertOperation(op *Operation) error
	SetOperationOutcome(operationID, outcome string) error
	// LockReservation returns nil when the (order, goods) row does not exist.
	LockReservation(orderID string, goodsID uint64) (*Reservation, error)
	// LockReservations returns the order's rows in goods_id order.
	LockReservations(orderID string) ([]Reservation, error)
	// ReservedGoods lists the goods ids an order has rows for, without
	// locking, so a Commit can lock their inventory rows first.
	ReservedGoods(orderID string) ([]uint64, error)
	InsertReservation(r *Reservation) error
	SetReservationState(id uint64, state string) error
	// LockStock returns ok=false when the goods has no inventory row.
	LockStock(goodsID uint64) (stock Stock, ok bool, err error)
	// MoveStock applies deltas to one inventory row.
	MoveStock(goodsID uint64, available, reserved, sold int64) error
	// Sellable reports whether the goods exists, is not deleted and is on shelf.
	Sellable(goodsID uint64) (bool, error)
}
