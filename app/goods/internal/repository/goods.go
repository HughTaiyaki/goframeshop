// Package repository implements the persistence seams with GoFrame DAOs.
//
// goods_info and category_info have a deleted_at column, so GoFrame adds
// "deleted_at IS NULL" to every query on them automatically.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

type goodsRepository struct{}

// NewGoods returns the MySQL-backed product repository.
func NewGoods() service.GoodsRepository {
	return &goodsRepository{}
}

// Create writes the product and its inventory row together: a product
// without an inventory row could never be reserved.
func (r *goodsRepository) Create(ctx context.Context, g *entity.GoodsInfo, initialStock uint32) (uint64, error) {
	var id int64
	err := dao.GoodsInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		data := goodsColumns(g)
		data.Status = g.Status
		var err error
		id, err = dao.GoodsInfo.Ctx(ctx).TX(tx).Data(data).InsertAndGetId()
		if err != nil {
			return err
		}
		_, err = dao.Inventory.Ctx(ctx).TX(tx).Data(do.Inventory{
			GoodsId:   id,
			Available: initialStock,
		}).Insert()
		return err
	})
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *goodsRepository) FindByID(ctx context.Context, id uint64) (*entity.GoodsInfo, error) {
	one, err := dao.GoodsInfo.Ctx(ctx).Where(dao.GoodsInfo.Columns().Id, id).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var goods entity.GoodsInfo
	if err := one.Struct(&goods); err != nil {
		return nil, err
	}
	return &goods, nil
}

// Update, SetStatus and SoftDelete each write the change and a goods.changed
// event in one transaction (design §8.1): banner slots and any later
// consumer learn about it exactly when it commits, and never about a change
// that rolled back.
func (r *goodsRepository) Update(ctx context.Context, g *entity.GoodsInfo) error {
	return changeGoods(ctx, g.Id, func(m *gdb.Model) error {
		_, err := m.Data(goodsColumns(g)).Where(dao.GoodsInfo.Columns().Id, g.Id).Update()
		return err
	})
}

func (r *goodsRepository) SetStatus(ctx context.Context, id uint64, status uint) error {
	return changeGoods(ctx, id, func(m *gdb.Model) error {
		_, err := m.Data(do.GoodsInfo{Status: status}).Where(dao.GoodsInfo.Columns().Id, id).Update()
		return err
	})
}

func (r *goodsRepository) SoftDelete(ctx context.Context, id uint64) error {
	return changeGoods(ctx, id, func(m *gdb.Model) error {
		_, err := m.Data(gdb.Map{dao.GoodsInfo.Columns().DeletedAt: gtime.Now()}).
			Where(dao.GoodsInfo.Columns().Id, id).Update()
		return err
	})
}

// changeGoods runs write and then appends goods.changed describing the row
// as it now is. The write locks the row, so events of one goods are appended
// in commit order and their outbox ids are increasing versions.
func changeGoods(ctx context.Context, id uint64, write func(m *gdb.Model) error) error {
	return dao.GoodsInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := write(dao.GoodsInfo.Ctx(ctx).TX(tx)); err != nil {
			return err
		}
		var row *entity.GoodsInfo
		// Unscoped: a deleted goods is still described (as deleted).
		err := dao.GoodsInfo.Ctx(ctx).TX(tx).Unscoped().Where(dao.GoodsInfo.Columns().Id, id).Scan(&row)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if row == nil {
			return nil // no such goods: nothing changed
		}
		return eventbus.Append(ctx, tx, eventbus.Event{
			Type:        eventbus.TypeGoodsChanged,
			AggregateID: strconv.FormatUint(id, 10),
			Payload: eventbus.GoodsChanged{
				GoodsID: id,
				Name:    row.Name,
				OnShelf: row.Status == service.StatusOnShelf && row.DeletedAt == nil,
				Deleted: row.DeletedAt != nil,
			},
		})
	})
}

// List returns one page, newest first (hot: by sort descending, then newest
// first). The id always breaks ties, so paging is deterministic.
func (r *goodsRepository) List(ctx context.Context, filter service.GoodsFilter, page, size int) ([]*entity.GoodsInfo, error) {
	model := r.filtered(ctx, filter)
	if filter.HotOnly {
		model = model.OrderDesc(dao.GoodsInfo.Columns().Sort)
	}
	all, err := model.
		OrderDesc(dao.GoodsInfo.Columns().Id).
		Page(page, size).
		All()
	if err != nil {
		return nil, err
	}
	goods := make([]*entity.GoodsInfo, 0, all.Len())
	if err := all.Structs(&goods); err != nil {
		return nil, err
	}
	return goods, nil
}

func (r *goodsRepository) Count(ctx context.Context, filter service.GoodsFilter) (int, error) {
	return r.filtered(ctx, filter).Count()
}

// filtered matches a category at any of the three levels, a shelf status and
// the hot flag.
func (r *goodsRepository) filtered(ctx context.Context, filter service.GoodsFilter) *gdb.Model {
	columns := dao.GoodsInfo.Columns()
	model := dao.GoodsInfo.Ctx(ctx)
	switch filter.Shelf {
	case service.ShelfOn:
		model = model.Where(columns.Status, service.StatusOnShelf)
	case service.ShelfOff:
		model = model.Where(columns.Status, service.StatusOffShelf)
	}
	if filter.HotOnly {
		model = model.WhereGT(columns.Sort, 0)
	}
	if filter.CategoryID == 0 {
		return model
	}
	return model.Where(model.Builder().
		Where(columns.Level1CategoryId, filter.CategoryID).
		WhereOr(columns.Level2CategoryId, filter.CategoryID).
		WhereOr(columns.Level3CategoryId, filter.CategoryID))
}

// goodsColumns lists the columns an edit may write. Status changes only
// through SetStatus, and stock lives in the inventory table.
func goodsColumns(g *entity.GoodsInfo) do.GoodsInfo {
	return do.GoodsInfo{
		Name:             g.Name,
		PriceCents:       g.PriceCents,
		Level1CategoryId: g.Level1CategoryId,
		Level2CategoryId: g.Level2CategoryId,
		Level3CategoryId: g.Level3CategoryId,
		Brand:            g.Brand,
		Tags:             g.Tags,
		DetailInfo:       g.DetailInfo,
		Sort:             g.Sort,
	}
}
