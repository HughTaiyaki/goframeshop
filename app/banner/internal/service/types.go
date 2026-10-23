// Package service defines the contracts of the banner service.
package service

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
)

const MaxLinkBytes = 512

type Rotation struct {
	ID        uint64
	FileID    uint64
	PicURL    string
	Link      string
	Sort      uint32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RotationInput struct {
	FileID uint64
	Link   string
	Sort   uint32
}

type Position struct {
	ID        uint64
	FileID    uint64
	PicURL    string
	GoodsID   uint64
	GoodsName string
	// GoodsOnShelf turns false when goods.changed says the goods is off
	// shelf or deleted.
	GoodsOnShelf bool
	Link         string
	Sort         uint32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PositionInput struct {
	FileID  uint64
	GoodsID uint64
	Link    string
	Sort    uint32
}

type IRotation interface {
	List(ctx context.Context, page, size int) ([]*Rotation, int, error)
	Create(ctx context.Context, in *RotationInput) (uint64, error)
	Update(ctx context.Context, id uint64, in *RotationInput) error
	Delete(ctx context.Context, id uint64) error
}

type IPosition interface {
	// List pages through the slots; availableOnly leaves out those whose
	// goods cannot be bought.
	List(ctx context.Context, page, size int, availableOnly bool) ([]*Position, int, error)
	Create(ctx context.Context, in *PositionInput) (uint64, error)
	Update(ctx context.Context, id uint64, in *PositionInput) error
	Delete(ctx context.Context, id uint64) error
}

// Files resolves a file id through the file service with the caller's
// administrator token. ErrNotFound when the file does not exist.
type Files interface {
	URL(ctx context.Context, fileID uint64) (string, error)
}

// Catalog reads a goods name through the goods service with the caller's
// administrator token, so goods still off shelf can be prepared too.
// ErrNotFound when the goods does not exist or was deleted.
type Catalog interface {
	GoodsName(ctx context.Context, goodsID uint64) (string, error)
}

// RotationRepository reads skip soft-deleted rows.
type RotationRepository interface {
	Create(ctx context.Context, r *Rotation) (uint64, error)
	Exists(ctx context.Context, id uint64) (bool, error)
	Update(ctx context.Context, r *Rotation) error
	SoftDelete(ctx context.Context, id uint64) error
	List(ctx context.Context, page, size int) ([]*Rotation, error)
	Count(ctx context.Context) (int, error)
}

// PositionRepository reads skip soft-deleted rows.
type PositionRepository interface {
	Create(ctx context.Context, p *Position) (uint64, error)
	Exists(ctx context.Context, id uint64) (bool, error)
	Update(ctx context.Context, p *Position) error
	SoftDelete(ctx context.Context, id uint64) error
	List(ctx context.Context, page, size int, availableOnly bool) ([]*Position, error)
	Count(ctx context.Context, availableOnly bool) (int, error)
}
