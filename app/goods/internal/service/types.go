// Package service defines the application contracts of the goods service.
//
// Controllers depend only on these interfaces, never on MySQL, Redis or the
// file service, so each of those can be replaced by a fake in tests.
package service

import (
	"errors"
	"time"
)

// Sentinel errors. The controller layer maps them to gRPC status codes.
var (
	ErrNoIdentity         = errors.New("caller identity is missing")
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrFailedPrecondition = errors.New("resource is still in use")
	// ErrInsufficientStock: AdjustStock would take available stock below 0.
	ErrInsufficientStock = errors.New("not enough available stock")
	// ErrIdempotencyConflict: an operation id was reused with different
	// parameters. Retrying cannot fix it; the caller has a bug.
	ErrIdempotencyConflict = errors.New("operation id reused with different parameters")
)

// Goods shelf status, stored in goods_info.status.
const (
	StatusOnShelf  uint = 1
	StatusOffShelf uint = 2
)

// ShelfFilter selects goods by shelf status in a list.
type ShelfFilter int

const (
	ShelfAll ShelfFilter = iota
	ShelfOn
	ShelfOff
)

// GoodsFilter selects the goods of a list. The zero value is every goods,
// newest first.
type GoodsFilter struct {
	// CategoryID matches a category at any level; 0 means all.
	CategoryID uint64
	Shelf      ShelfFilter
	// HotOnly keeps goods with Sort > 0 and orders them by Sort descending
	// (then newest first) instead of newest first.
	HotOnly bool
}

// Limits shared by validation and tests. They match the column sizes.
const (
	MaxGoodsNameRunes    = 200
	MaxBrandRunes        = 30
	MaxTagsRunes         = 255
	MaxDetailRunes       = 16000
	MaxCategoryNameRunes = 30
	MaxCategoryLevel     = 3
	// MaxHotSort caps the hot weight (goods_info.sort is SMALLINT UNSIGNED).
	MaxHotSort = 9999
)

// Goods is the full product, including images. It is also what the detail
// cache stores, so every field is plain data that survives JSON, except the
// stock numbers: they change on every reservation, so they are read live from
// the inventory ledger and never cached (json:"-").
type Goods struct {
	ID               uint64       `json:"id"`
	Name             string       `json:"name"`
	PriceCents       uint64       `json:"priceCents"`
	Level1CategoryID uint64       `json:"level1CategoryId"`
	Level2CategoryID uint64       `json:"level2CategoryId"`
	Level3CategoryID uint64       `json:"level3CategoryId"`
	Brand            string       `json:"brand"`
	OnShelf          bool         `json:"onShelf"`
	Sort             uint32       `json:"sort"`
	Tags             string       `json:"tags"`
	DetailInfo       string       `json:"detailInfo"`
	Images           []GoodsImage `json:"images"`
	CreatedAt        time.Time    `json:"createdAt"`
	UpdatedAt        time.Time    `json:"updatedAt"`

	Stock Stock `json:"-"`
}

// Stock is a product's row in the inventory ledger.
type Stock struct {
	Available uint32
	Reserved  uint32
	Sold      uint32
}

// GoodsImage is one image of a product, ordered by Sort ascending.
type GoodsImage struct {
	ID     uint64 `json:"id"`
	FileID uint64 `json:"fileId"`
	URL    string `json:"url"`
	Sort   uint32 `json:"sort"`
}

// GoodsSummary is one row of a product list.
type GoodsSummary struct {
	ID         uint64
	Name       string
	PriceCents uint64
	Brand      string
	OnShelf    bool
	Sort       uint32
	Tags       string
	CoverURL   string
	Stock      Stock
}

// GoodsInput holds the editable fields. Stock and sale are absent on purpose:
// stock changes through AdjustStock and reservations, sale through orders.
type GoodsInput struct {
	Name             string
	PriceCents       uint64
	Level1CategoryID uint64
	Level2CategoryID uint64
	Level3CategoryID uint64
	Brand            string
	Tags             string
	DetailInfo       string
	// Sort is the hot weight: 0 not hot, larger first.
	Sort uint32
}

// Category is one node of the three-level tree.
type Category struct {
	ID         uint64    `json:"id"`
	ParentID   uint64    `json:"parentId"`
	Level      uint32    `json:"level"`
	Name       string    `json:"name"`
	IconFileID uint64    `json:"iconFileId"`
	IconURL    string    `json:"iconUrl"`
	Sort       uint32    `json:"sort"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// CategoryInput holds the writable fields of a category. ParentID is only
// used on create: a category never moves.
type CategoryInput struct {
	ParentID   uint64
	Name       string
	IconFileID uint64
	Sort       uint32
}
