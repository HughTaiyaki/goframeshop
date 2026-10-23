package service

import (
	"context"
	"errors"
	"time"
)

// Cart limits. A row holds what one order line may hold.
const (
	MaxCartQuantity = 99
	MaxCartLines    = 100
)

// ErrCartLimit: the add would take a row above MaxCartQuantity, or the cart
// already has MaxCartLines rows. Nothing was changed.
var ErrCartLimit = errors.New("cart limit reached")

// CartItem is one row of a shopper's cart.
type CartItem struct {
	ID        uint64
	UserID    uint64
	GoodsID   uint64
	Quantity  uint32
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CartLine is a row together with its goods as they are now. A goods off
// shelf or deleted has Available false; a deleted one has no name or price.
type CartLine struct {
	CartItem
	GoodsName  string
	PriceCents uint64
	CoverURL   string
	Available  bool
	Stock      uint32
}

// ICart is the cart application service. userID is always the caller,
// taken from the verified token; every method acts on that user's rows only.
type ICart interface {
	List(ctx context.Context, userID uint64, page, size int) ([]*CartLine, int, error)
	Add(ctx context.Context, userID, goodsID uint64, quantity uint32) (*CartLine, error)
	Update(ctx context.Context, userID, itemID uint64, quantity uint32) (*CartLine, error)
	// Delete removes the caller's rows among itemIDs and reports how many.
	Delete(ctx context.Context, userID uint64, itemIDs []uint64) (int, error)
}

// CartRepository is the persistence seam for cart_item. Every statement is
// limited to userID, so a wrong id can never touch another shopper's row.
type CartRepository interface {
	// List returns one page, newest row first.
	List(ctx context.Context, userID uint64, page, size int) ([]*CartItem, error)
	Count(ctx context.Context, userID uint64) (int, error)
	// Find returns nil when the row does not exist or is not userID's.
	Find(ctx context.Context, userID, itemID uint64) (*CartItem, error)
	FindByGoods(ctx context.Context, userID, goodsID uint64) (*CartItem, error)
	// Add inserts the row, or adds quantity to the existing one, in one
	// statement. It returns ErrCartLimit, changing nothing, when the result
	// would exceed max.
	Add(ctx context.Context, userID, goodsID uint64, quantity, max uint32) (*CartItem, error)
	// SetQuantity reports whether the row exists and is userID's.
	SetQuantity(ctx context.Context, userID, itemID uint64, quantity uint32) (bool, error)
	Delete(ctx context.Context, userID uint64, itemIDs []uint64) (int, error)
}
