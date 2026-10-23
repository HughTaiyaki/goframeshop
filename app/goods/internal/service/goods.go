package service

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
)

// IGoods is the product application service.
//
// A public caller (viewAll=false) only ever sees goods on shelf; an
// off-shelf product is NotFound to them, exactly like a deleted one.
type IGoods interface {
	List(ctx context.Context, filter GoodsFilter, page, size int) ([]*GoodsSummary, int, error)
	Get(ctx context.Context, goodsID uint64, viewAll bool) (*Goods, error)
	// Create stores new goods OFF shelf with the given starting stock.
	Create(ctx context.Context, in *GoodsInput, initialStock uint32) (uint64, error)
	Update(ctx context.Context, goodsID uint64, in *GoodsInput) error
	Delete(ctx context.Context, goodsID uint64) error
	AddImage(ctx context.Context, goodsID, fileID uint64, sort uint32) (uint64, error)
	RemoveImage(ctx context.Context, imageID uint64) error
	SetShelf(ctx context.Context, goodsID uint64, onShelf bool) error
	AdjustStock(ctx context.Context, goodsID uint64, delta int64) (*Stock, error)
}

// GoodsRepository is the persistence seam for goods_info. Every read skips
// soft-deleted rows.
type GoodsRepository interface {
	// Create inserts the product and its inventory row in one transaction.
	Create(ctx context.Context, g *entity.GoodsInfo, initialStock uint32) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.GoodsInfo, error)
	// FindByIDs returns the goods that exist (not deleted) among ids.
	FindByIDs(ctx context.Context, ids []uint64) (map[uint64]*entity.GoodsInfo, error)
	Update(ctx context.Context, g *entity.GoodsInfo) error
	SoftDelete(ctx context.Context, id uint64) error
	SetStatus(ctx context.Context, id uint64, status uint) error
	// List and Count apply the filter; List orders as GoodsFilter describes.
	List(ctx context.Context, filter GoodsFilter, page, size int) ([]*entity.GoodsInfo, error)
	Count(ctx context.Context, filter GoodsFilter) (int, error)
}

// StockRepository reads and adjusts the inventory ledger outside the
// reservation flow.
type StockRepository interface {
	Find(ctx context.Context, goodsID uint64) (Stock, error)
	ByGoods(ctx context.Context, goodsIDs []uint64) (map[uint64]Stock, error)
	// Adjust changes available stock by delta in one conditional UPDATE and
	// returns ErrInsufficientStock if the result would be negative.
	Adjust(ctx context.Context, goodsID uint64, delta int64) (Stock, error)
}

// ImageRepository is the persistence seam for goods_images. Create returns
// ErrAlreadyExists when the file is already attached to the product.
type ImageRepository interface {
	Create(ctx context.Context, img *entity.GoodsImages) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.GoodsImages, error)
	Delete(ctx context.Context, id uint64) error
	ListByGoods(ctx context.Context, goodsID uint64) ([]*entity.GoodsImages, error)
	// Covers returns the first image URL by sort order for each product that
	// has one, in a single query.
	Covers(ctx context.Context, goodsIDs []uint64) (map[uint64]string, error)
}

// FileLookup resolves a file id through the file service. It returns
// ErrNotFound when the file does not exist.
type FileLookup interface {
	URL(ctx context.Context, fileID uint64) (string, error)
}
