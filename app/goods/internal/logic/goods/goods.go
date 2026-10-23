// Package goods implements the product application service.
//
// Reads of a single product go through the cache (cache-aside):
//
//	GetGoods → cache hit?            → return it
//	         → cached as absent?     → NotFound, without touching MySQL
//	         → miss → MySQL (one load per id at a time) → fill cache → return
//
// Writes go to MySQL first and delete the cached entry afterwards. Deleting
// instead of overwriting means a write never has to rebuild the whole cached
// product, and the next read loads the fresh version.
//
// The cached product is the same for every caller and includes its shelf
// status; whether the caller may see an off-shelf product is decided after
// the cache. Stock numbers are never cached: they change on every
// reservation, and are read live from the inventory ledger (one primary-key
// lookup), so the cache is not invalidated by every order.
package goods

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"golang.org/x/sync/singleflight"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type sGoods struct {
	goods      service.GoodsRepository
	stocks     service.StockRepository
	images     service.ImageRepository
	categories service.CategoryRepository
	files      service.FileLookup
	cache      service.Cache
	// loads collapses concurrent cache misses for the same product into one
	// MySQL query, so a popular product expiring does not stampede the DB.
	loads singleflight.Group
}

func New(goods service.GoodsRepository, stocks service.StockRepository, images service.ImageRepository,
	categories service.CategoryRepository, files service.FileLookup, cache service.Cache) service.IGoods {
	return &sGoods{goods: goods, stocks: stocks, images: images, categories: categories, files: files, cache: cache}
}

func (s *sGoods) List(ctx context.Context, filter service.GoodsFilter, page, size int) ([]*service.GoodsSummary, int, error) {
	total, err := s.goods.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	records, err := s.goods.List(ctx, filter, page, size)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint64, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.Id)
	}
	covers, err := s.images.Covers(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	stocks, err := s.stocks.ByGoods(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*service.GoodsSummary, 0, len(records))
	for _, record := range records {
		items = append(items, &service.GoodsSummary{
			ID:         record.Id,
			Name:       record.Name,
			PriceCents: record.PriceCents,
			Brand:      record.Brand,
			OnShelf:    record.Status == service.StatusOnShelf,
			Sort:       uint32(record.Sort),
			Tags:       record.Tags,
			CoverURL:   covers[record.Id],
			Stock:      stocks[record.Id],
		})
	}
	return items, total, nil
}

func (s *sGoods) Get(ctx context.Context, goodsID uint64, viewAll bool) (*service.Goods, error) {
	goods, err := s.cachedOrLoaded(ctx, goodsID)
	if err != nil {
		return nil, err
	}
	// Off shelf looks exactly like deleted to the public.
	if !goods.OnShelf && !viewAll {
		return nil, service.ErrNotFound
	}
	// Copy before filling in stock, so the shared cached value never carries
	// one caller's live numbers to another.
	out := *goods
	out.Stock, err = s.stocks.Find(ctx, goodsID)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *sGoods) cachedOrLoaded(ctx context.Context, goodsID uint64) (*service.Goods, error) {
	cached, found, err := s.cache.GetGoods(ctx, goodsID)
	switch {
	case err != nil:
		// The cache is an optimisation. When it fails, serve from MySQL.
		g.Log().Warningf(ctx, "goods cache read failed, falling back to MySQL: %v", err)
	case found && cached == nil:
		return nil, service.ErrNotFound
	case found:
		return cached, nil
	}

	loaded, err, _ := s.loads.Do(strconv.FormatUint(goodsID, 10), func() (any, error) {
		return s.load(ctx, goodsID)
	})
	if err != nil {
		return nil, err
	}
	goods := loaded.(*service.Goods)
	if goods == nil {
		if err := s.cache.SetGoodsAbsent(ctx, goodsID); err != nil {
			g.Log().Warningf(ctx, "goods cache write failed: %v", err)
		}
		return nil, service.ErrNotFound
	}
	if err := s.cache.SetGoods(ctx, goods); err != nil {
		g.Log().Warningf(ctx, "goods cache write failed: %v", err)
	}
	return goods, nil
}

// load reads a product and its images from MySQL. A missing product is
// (nil, nil) so the caller can cache the absence.
func (s *sGoods) load(ctx context.Context, goodsID uint64) (*service.Goods, error) {
	record, err := s.goods.FindByID(ctx, goodsID)
	if err != nil || record == nil {
		return nil, err
	}
	images, err := s.images.ListByGoods(ctx, goodsID)
	if err != nil {
		return nil, err
	}
	return toDomain(record, images), nil
}

func (s *sGoods) Create(ctx context.Context, in *service.GoodsInput, initialStock uint32) (uint64, error) {
	record, err := s.validate(ctx, in)
	if err != nil {
		return 0, err
	}
	// New goods start off shelf: an administrator completes them (images,
	// stock) and then publishes them with SetShelf.
	record.Status = service.StatusOffShelf
	id, err := s.goods.Create(ctx, record, initialStock)
	if err != nil {
		return 0, err
	}
	// Someone may have asked for this id before it existed, leaving an
	// "absent" marker that would hide the new product until it expired.
	s.invalidate(ctx, id)
	return id, nil
}

func (s *sGoods) Update(ctx context.Context, goodsID uint64, in *service.GoodsInput) error {
	if _, err := s.existing(ctx, goodsID); err != nil {
		return err
	}
	record, err := s.validate(ctx, in)
	if err != nil {
		return err
	}
	record.Id = goodsID
	if err := s.goods.Update(ctx, record); err != nil {
		return err
	}
	s.invalidate(ctx, goodsID)
	return nil
}

func (s *sGoods) Delete(ctx context.Context, goodsID uint64) error {
	if _, err := s.existing(ctx, goodsID); err != nil {
		return err
	}
	if err := s.goods.SoftDelete(ctx, goodsID); err != nil {
		return err
	}
	s.invalidate(ctx, goodsID)
	return nil
}

func (s *sGoods) AddImage(ctx context.Context, goodsID, fileID uint64, sort uint32) (uint64, error) {
	if _, err := s.existing(ctx, goodsID); err != nil {
		return 0, err
	}
	// Only a file the file service knows can be attached, and its URL is
	// copied here so that reading a product never calls another service.
	url, err := s.files.URL(ctx, fileID)
	if errors.Is(err, service.ErrNotFound) {
		// The product exists; it is the file reference that is wrong.
		return 0, service.ErrInvalidArgument
	}
	if err != nil {
		return 0, err
	}
	id, err := s.images.Create(ctx, &entity.GoodsImages{GoodsId: goodsID, FileId: fileID, Url: url, Sort: uint(sort)})
	if err != nil {
		return 0, err
	}
	s.invalidate(ctx, goodsID)
	return id, nil
}

func (s *sGoods) RemoveImage(ctx context.Context, imageID uint64) error {
	image, err := s.images.FindByID(ctx, imageID)
	if err != nil {
		return err
	}
	if image == nil {
		return service.ErrNotFound
	}
	if err := s.images.Delete(ctx, imageID); err != nil {
		return err
	}
	s.invalidate(ctx, image.GoodsId)
	return nil
}

func (s *sGoods) SetShelf(ctx context.Context, goodsID uint64, onShelf bool) error {
	if _, err := s.existing(ctx, goodsID); err != nil {
		return err
	}
	status := service.StatusOffShelf
	if onShelf {
		status = service.StatusOnShelf
	}
	if err := s.goods.SetStatus(ctx, goodsID, status); err != nil {
		return err
	}
	s.invalidate(ctx, goodsID)
	return nil
}

// AdjustStock changes available stock only. Reserved and sold stock belong to
// orders and move only through the reservation flow.
func (s *sGoods) AdjustStock(ctx context.Context, goodsID uint64, delta int64) (*service.Stock, error) {
	if delta == 0 {
		return nil, service.ErrInvalidArgument
	}
	if _, err := s.existing(ctx, goodsID); err != nil {
		return nil, err
	}
	stock, err := s.stocks.Adjust(ctx, goodsID, delta)
	if err != nil {
		return nil, err
	}
	return &stock, nil
}

func (s *sGoods) existing(ctx context.Context, goodsID uint64) (*entity.GoodsInfo, error) {
	record, err := s.goods.FindByID(ctx, goodsID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, service.ErrNotFound
	}
	return record, nil
}

// invalidate drops the cached product after a successful write. A failure is
// logged, not returned: MySQL already holds the new data, and the cached copy
// expires on its own within its TTL.
func (s *sGoods) invalidate(ctx context.Context, goodsID uint64) {
	if err := s.cache.DeleteGoods(ctx, goodsID); err != nil {
		g.Log().Warningf(ctx, "goods cache delete failed for %d, stale for up to one TTL: %v", goodsID, err)
	}
}

// validate checks the fields and the category chain, and returns the row to
// write. The chain must be level1 → level2 → level3, each the parent of the
// next; level2 and level3 are optional, but level3 needs level2.
func (s *sGoods) validate(ctx context.Context, in *service.GoodsInput) (*entity.GoodsInfo, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > service.MaxGoodsNameRunes ||
		in.PriceCents == 0 ||
		utf8.RuneCountInString(in.Brand) > service.MaxBrandRunes ||
		utf8.RuneCountInString(in.Tags) > service.MaxTagsRunes ||
		utf8.RuneCountInString(in.DetailInfo) > service.MaxDetailRunes ||
		in.Sort > service.MaxHotSort ||
		in.Level1CategoryID == 0 ||
		(in.Level3CategoryID != 0 && in.Level2CategoryID == 0) {
		return nil, service.ErrInvalidArgument
	}
	chain := []struct {
		id, parent uint64
		level      uint
	}{
		{in.Level1CategoryID, 0, 1},
		{in.Level2CategoryID, in.Level1CategoryID, 2},
		{in.Level3CategoryID, in.Level2CategoryID, 3},
	}
	for _, link := range chain {
		if link.id == 0 {
			break
		}
		category, err := s.categories.FindByID(ctx, link.id)
		if err != nil {
			return nil, err
		}
		if category == nil || category.Level != link.level || category.ParentId != link.parent {
			return nil, service.ErrInvalidArgument
		}
	}
	return &entity.GoodsInfo{
		Name:             name,
		PriceCents:       in.PriceCents,
		Level1CategoryId: in.Level1CategoryID,
		Level2CategoryId: in.Level2CategoryID,
		Level3CategoryId: in.Level3CategoryID,
		Brand:            strings.TrimSpace(in.Brand),
		Tags:             strings.TrimSpace(in.Tags),
		DetailInfo:       in.DetailInfo,
		Sort:             uint(in.Sort),
	}, nil
}

func toDomain(record *entity.GoodsInfo, images []*entity.GoodsImages) *service.Goods {
	goods := &service.Goods{
		ID:               record.Id,
		Name:             record.Name,
		PriceCents:       record.PriceCents,
		Level1CategoryID: record.Level1CategoryId,
		Level2CategoryID: record.Level2CategoryId,
		Level3CategoryID: record.Level3CategoryId,
		Brand:            record.Brand,
		OnShelf:          record.Status == service.StatusOnShelf,
		Sort:             uint32(record.Sort),
		Tags:             record.Tags,
		DetailInfo:       record.DetailInfo,
		Images:           make([]service.GoodsImage, 0, len(images)),
	}
	if record.CreatedAt != nil {
		goods.CreatedAt = record.CreatedAt.Time
	}
	if record.UpdatedAt != nil {
		goods.UpdatedAt = record.UpdatedAt.Time
	}
	for _, image := range images {
		goods.Images = append(goods.Images, service.GoodsImage{
			ID: image.Id, FileID: image.FileId, URL: image.Url, Sort: uint32(image.Sort),
		})
	}
	return goods
}
