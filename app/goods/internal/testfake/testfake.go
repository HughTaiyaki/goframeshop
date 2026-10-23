// Package testfake holds in-memory stand-ins for the goods service's
// dependencies, shared by the logic tests. Each fake records enough to let a
// test assert what happened, such as how many times MySQL was read.
package testfake

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

// Goods is an in-memory goods_info table. Create also creates the
// inventory row in Stocks, as the real repository does in one transaction.
type Goods struct {
	mu     sync.Mutex
	Rows   map[uint64]*entity.GoodsInfo
	Stocks *Stocks
	nextID uint64
	// Finds counts FindByID calls, i.e. MySQL reads of a single product.
	Finds atomic.Int64
	// FindGate, when set, blocks FindByID until it is closed, so a test can
	// pile up concurrent cache misses.
	FindGate chan struct{}
}

func NewGoods() *Goods { return &Goods{Rows: map[uint64]*entity.GoodsInfo{}, Stocks: NewStocks()} }

func (f *Goods) Create(_ context.Context, g *entity.GoodsInfo, initialStock uint32) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	stored := *g
	stored.Id = f.nextID
	stored.CreatedAt, stored.UpdatedAt = gtime.Now(), gtime.Now()
	f.Rows[f.nextID] = &stored
	f.Stocks.Set(f.nextID, service.Stock{Available: initialStock})
	return f.nextID, nil
}

func (f *Goods) SetStatus(_ context.Context, id uint64, status uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Rows[id].Status = status
	return nil
}

func (f *Goods) FindByID(_ context.Context, id uint64) (*entity.GoodsInfo, error) {
	f.Finds.Add(1)
	if f.FindGate != nil {
		<-f.FindGate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.Rows[id]
	if !ok || row.DeletedAt != nil {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (f *Goods) Update(_ context.Context, g *entity.GoodsInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.Rows[g.Id]
	status, created := row.Status, row.CreatedAt
	*row = *g
	row.Status, row.CreatedAt, row.UpdatedAt = status, created, gtime.Now()
	return nil
}

func (f *Goods) SoftDelete(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Rows[id].DeletedAt = gtime.Now()
	return nil
}

func (f *Goods) matching(filter service.GoodsFilter) []*entity.GoodsInfo {
	out := []*entity.GoodsInfo{}
	for _, row := range f.Rows {
		if row.DeletedAt != nil ||
			(filter.Shelf == service.ShelfOn && row.Status != service.StatusOnShelf) ||
			(filter.Shelf == service.ShelfOff && row.Status != service.StatusOffShelf) ||
			(filter.HotOnly && row.Sort == 0) {
			continue
		}
		c := filter.CategoryID
		if c == 0 || row.Level1CategoryId == c || row.Level2CategoryId == c || row.Level3CategoryId == c {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if filter.HotOnly && out[i].Sort != out[j].Sort {
			return out[i].Sort > out[j].Sort
		}
		return out[i].Id > out[j].Id
	})
	return out
}

func (f *Goods) List(_ context.Context, filter service.GoodsFilter, page, size int) ([]*entity.GoodsInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	all := f.matching(filter)
	start := (page - 1) * size
	if start >= len(all) {
		return []*entity.GoodsInfo{}, nil
	}
	return all[start:min(start+size, len(all))], nil
}

func (f *Goods) Count(_ context.Context, filter service.GoodsFilter) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.matching(filter)), nil
}

// Stocks is an in-memory inventory table.
type Stocks struct {
	mu   sync.Mutex
	Rows map[uint64]service.Stock
}

func NewStocks() *Stocks { return &Stocks{Rows: map[uint64]service.Stock{}} }

func (f *Stocks) Set(goodsID uint64, stock service.Stock) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Rows[goodsID] = stock
}

func (f *Stocks) Find(_ context.Context, goodsID uint64) (service.Stock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Rows[goodsID], nil
}

func (f *Stocks) ByGoods(_ context.Context, ids []uint64) (map[uint64]service.Stock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uint64]service.Stock{}
	for _, id := range ids {
		if stock, ok := f.Rows[id]; ok {
			out[id] = stock
		}
	}
	return out, nil
}

func (f *Stocks) Adjust(_ context.Context, goodsID uint64, delta int64) (service.Stock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stock := f.Rows[goodsID]
	if int64(stock.Available)+delta < 0 {
		return service.Stock{}, service.ErrInsufficientStock
	}
	stock.Available = uint32(int64(stock.Available) + delta)
	f.Rows[goodsID] = stock
	return stock, nil
}

// Images is an in-memory goods_images table with its unique key.
type Images struct {
	Rows   map[uint64]*entity.GoodsImages
	nextID uint64
}

func NewImages() *Images { return &Images{Rows: map[uint64]*entity.GoodsImages{}} }

func (f *Images) Create(_ context.Context, img *entity.GoodsImages) (uint64, error) {
	for _, row := range f.Rows {
		if row.GoodsId == img.GoodsId && row.FileId == img.FileId {
			return 0, service.ErrAlreadyExists
		}
	}
	f.nextID++
	stored := *img
	stored.Id = f.nextID
	f.Rows[f.nextID] = &stored
	return f.nextID, nil
}

func (f *Images) FindByID(_ context.Context, id uint64) (*entity.GoodsImages, error) {
	if row, ok := f.Rows[id]; ok {
		copied := *row
		return &copied, nil
	}
	return nil, nil
}

func (f *Images) Delete(_ context.Context, id uint64) error {
	delete(f.Rows, id)
	return nil
}

func (f *Images) ListByGoods(_ context.Context, goodsID uint64) ([]*entity.GoodsImages, error) {
	out := []*entity.GoodsImages{}
	for _, row := range f.Rows {
		if row.GoodsId == goodsID {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return out[i].Id < out[j].Id
	})
	return out, nil
}

func (f *Images) Covers(ctx context.Context, goodsIDs []uint64) (map[uint64]string, error) {
	covers := map[uint64]string{}
	for _, id := range goodsIDs {
		images, _ := f.ListByGoods(ctx, id)
		if len(images) > 0 {
			covers[id] = images[0].Url
		}
	}
	return covers, nil
}

// Categories is an in-memory category_info table.
type Categories struct {
	Rows   map[uint64]*entity.CategoryInfo
	nextID uint64
	// ListAlls counts full-tree reads from MySQL.
	ListAlls int
}

func NewCategories() *Categories { return &Categories{Rows: map[uint64]*entity.CategoryInfo{}} }

func (f *Categories) Create(_ context.Context, c *entity.CategoryInfo) (uint64, error) {
	f.nextID++
	stored := *c
	stored.Id = f.nextID
	f.Rows[f.nextID] = &stored
	return f.nextID, nil
}

func (f *Categories) FindByID(_ context.Context, id uint64) (*entity.CategoryInfo, error) {
	if row, ok := f.Rows[id]; ok && row.DeletedAt == nil {
		copied := *row
		return &copied, nil
	}
	return nil, nil
}

func (f *Categories) Update(_ context.Context, c *entity.CategoryInfo) error {
	row := f.Rows[c.Id]
	row.Name, row.IconFileId, row.IconUrl, row.Sort = c.Name, c.IconFileId, c.IconUrl, c.Sort
	return nil
}

func (f *Categories) SoftDelete(_ context.Context, id uint64) error {
	f.Rows[id].DeletedAt = gtime.Now()
	return nil
}

func (f *Categories) children(parentID uint64) []*entity.CategoryInfo {
	out := []*entity.CategoryInfo{}
	for _, row := range f.Rows {
		if row.ParentId == parentID && row.DeletedAt == nil {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

func (f *Categories) ListByParent(_ context.Context, parentID uint64, page, size int) ([]*entity.CategoryInfo, error) {
	all := f.children(parentID)
	start := (page - 1) * size
	if start >= len(all) {
		return []*entity.CategoryInfo{}, nil
	}
	return all[start:min(start+size, len(all))], nil
}

func (f *Categories) CountByParent(_ context.Context, parentID uint64) (int, error) {
	return len(f.children(parentID)), nil
}

func (f *Categories) ListAll(_ context.Context) ([]*entity.CategoryInfo, error) {
	f.ListAlls++
	out := []*entity.CategoryInfo{}
	for _, row := range f.Rows {
		if row.DeletedAt == nil {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out, nil
}

// Files maps file ids to URLs, standing in for the file service.
type Files map[uint64]string

func (f Files) URL(_ context.Context, fileID uint64) (string, error) {
	if url, ok := f[fileID]; ok {
		return url, nil
	}
	return "", service.ErrNotFound
}

// ErrCacheDown is what a broken cache returns.
var ErrCacheDown = errors.New("cache is down")

// Cache is an in-memory service.Cache. Set Down to make every call fail.
type Cache struct {
	mu         sync.Mutex
	Goods      map[uint64]*service.Goods // nil value = cached as absent
	Categories []*service.Category
	HasCats    bool
	Down       bool
	Deletes    []uint64
}

func NewCache() *Cache { return &Cache{Goods: map[uint64]*service.Goods{}} }

func (c *Cache) GetGoods(_ context.Context, id uint64) (*service.Goods, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Down {
		return nil, false, ErrCacheDown
	}
	g, ok := c.Goods[id]
	return g, ok, nil
}

func (c *Cache) SetGoods(_ context.Context, g *service.Goods) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Down {
		return ErrCacheDown
	}
	c.Goods[g.ID] = g
	return nil
}

func (c *Cache) SetGoodsAbsent(_ context.Context, id uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Down {
		return ErrCacheDown
	}
	c.Goods[id] = nil
	return nil
}

func (c *Cache) DeleteGoods(_ context.Context, id uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Deletes = append(c.Deletes, id)
	if c.Down {
		return ErrCacheDown
	}
	delete(c.Goods, id)
	return nil
}

func (c *Cache) GetCategories(context.Context) ([]*service.Category, bool, error) {
	if c.Down {
		return nil, false, ErrCacheDown
	}
	return c.Categories, c.HasCats, nil
}

func (c *Cache) SetCategories(_ context.Context, categories []*service.Category) error {
	if c.Down {
		return ErrCacheDown
	}
	c.Categories, c.HasCats = categories, true
	return nil
}

func (c *Cache) DeleteCategories(context.Context) error {
	if c.Down {
		return ErrCacheDown
	}
	c.Categories, c.HasCats = nil, false
	return nil
}
