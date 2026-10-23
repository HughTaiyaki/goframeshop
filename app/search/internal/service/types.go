// Package service defines the contracts of the search service.
//
// The search index is a read-only projection of the goods database. Nothing
// here is a source of truth: the whole index can be dropped and rebuilt from
// MySQL at any time, and is, on every start.
package service

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrUnavailable: Elasticsearch did not answer. Retrying may help.
	ErrUnavailable = errors.New("search is temporarily unavailable")
)

// Limits shared by validation and tests.
const (
	MaxKeywordRunes = 100
	MaxBrandRunes   = 30
	MaxPageSize     = 100
	// MaxResultWindow is Elasticsearch's index.max_result_window default:
	// from+size beyond it is refused, so deep pages fail early here instead.
	MaxResultWindow = 10000
)

// GoodsDoc is one indexed product: only goods on shelf and not deleted.
type GoodsDoc struct {
	GoodsID          uint64    `json:"goods_id"`
	Name             string    `json:"name"`
	Tags             string    `json:"tags"`
	Brand            string    `json:"brand"`
	PriceCents       uint64    `json:"price_cents"`
	Level1CategoryID uint64    `json:"level1_category_id"`
	Level2CategoryID uint64    `json:"level2_category_id"`
	Level3CategoryID uint64    `json:"level3_category_id"`
	Stock            uint32    `json:"stock"`
	Sale             uint32    `json:"sale"`
	CoverURL         string    `json:"cover_url"`
	CreatedAt        time.Time `json:"created_at"`
}

type SortBy int

const (
	SortRelevance SortBy = iota
	SortPriceAsc
	SortPriceDesc
	SortSales
	SortNewest
)

// Query is a validated search request.
type Query struct {
	Keyword       string
	CategoryID    uint64
	Brand         string
	MinPriceCents uint64
	MaxPriceCents uint64
	Sort          SortBy
	Page, Size    int
}

// Hit is one result. HighlightedName is HTML-safe: Elasticsearch escapes the
// name and only the <em> markers are raw.
type Hit struct {
	GoodsDoc
	HighlightedName string
}

// ISearch is the search application service.
type ISearch interface {
	Search(ctx context.Context, q *Query) ([]Hit, int, error)
}

// Index is the Elasticsearch side.
type Index interface {
	Search(ctx context.Context, q *Query) ([]Hit, int, error)
	Upsert(ctx context.Context, doc *GoodsDoc) error
	Delete(ctx context.Context, goodsID uint64) error
	// Rebuild loads every document into a fresh physical index and then
	// moves the alias to it in one atomic step, so searches never see a
	// half-built index. Old physical indices are removed afterwards.
	Rebuild(ctx context.Context, load func(emit func(*GoodsDoc) error) error) (int, error)
}

// Source reads the current state of goods from MySQL.
type Source interface {
	// Load returns nil when the product should not be in the index: it does
	// not exist, is deleted, or is off shelf.
	Load(ctx context.Context, goodsID uint64) (*GoodsDoc, error)
	// LoadAll calls emit for every product that belongs in the index.
	LoadAll(ctx context.Context, emit func(*GoodsDoc) error) error
}
