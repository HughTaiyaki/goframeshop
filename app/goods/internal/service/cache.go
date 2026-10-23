package service

import "context"

// Cache is the read-through cache in front of MySQL.
//
// It is never the source of truth: every method may fail, and callers treat a
// failure as a cache miss and read MySQL instead. Writes go to MySQL first and
// then delete the cached entry.
type Cache interface {
	// GetGoods reports found=true with a nil product when the id is cached as
	// absent (negative caching against repeated lookups of missing ids).
	GetGoods(ctx context.Context, goodsID uint64) (goods *Goods, found bool, err error)
	SetGoods(ctx context.Context, goods *Goods) error
	SetGoodsAbsent(ctx context.Context, goodsID uint64) error
	DeleteGoods(ctx context.Context, goodsID uint64) error

	GetCategories(ctx context.Context) (categories []*Category, found bool, err error)
	SetCategories(ctx context.Context, categories []*Category) error
	DeleteCategories(ctx context.Context) error
}
