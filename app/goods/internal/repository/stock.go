package repository

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type stockRepository struct{}

// NewStock returns the MySQL-backed inventory reader and adjuster.
func NewStock() service.StockRepository {
	return &stockRepository{}
}

func (r *stockRepository) Find(ctx context.Context, goodsID uint64) (service.Stock, error) {
	stocks, err := r.ByGoods(ctx, []uint64{goodsID})
	return stocks[goodsID], err
}

func (r *stockRepository) ByGoods(ctx context.Context, goodsIDs []uint64) (map[uint64]service.Stock, error) {
	stocks := make(map[uint64]service.Stock, len(goodsIDs))
	if len(goodsIDs) == 0 {
		return stocks, nil
	}
	all, err := dao.Inventory.Ctx(ctx).WhereIn(dao.Inventory.Columns().GoodsId, goodsIDs).All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.Inventory
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		stocks[row.GoodsId] = toStock(row)
	}
	return stocks, nil
}

// Adjust is a single conditional UPDATE, so two administrators adjusting at
// the same time, or an adjustment racing a reservation, can never lose an
// update or drive available stock below zero.
func (r *stockRepository) Adjust(ctx context.Context, goodsID uint64, delta int64) (service.Stock, error) {
	columns := dao.Inventory.Columns()
	model := dao.Inventory.Ctx(ctx).
		Data(gdb.Map{columns.Available: gdb.Raw(fmt.Sprintf("`%s` + (%d)", columns.Available, delta))}).
		Where(columns.GoodsId, goodsID)
	if delta < 0 {
		model = model.WhereGTE(columns.Available, -delta)
	}
	result, err := model.Update()
	if err != nil {
		return service.Stock{}, err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return service.Stock{}, err
	} else if affected == 0 {
		return service.Stock{}, service.ErrInsufficientStock
	}
	return r.Find(ctx, goodsID)
}
