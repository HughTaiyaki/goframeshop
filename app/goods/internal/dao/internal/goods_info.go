// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// GoodsInfoDao is the data access object for the table goods_info.
type GoodsInfoDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  GoodsInfoColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// GoodsInfoColumns defines and stores column names for the table goods_info.
type GoodsInfoColumns struct {
	Id               string //
	Name             string //
	Status           string // 1 on shelf, 2 off shelf
	Sort             string // hot weight: 0 not hot, larger first
	PriceCents       string //
	Level1CategoryId string //
	Level2CategoryId string //
	Level3CategoryId string //
	Brand            string //
	Tags             string //
	DetailInfo       string //
	CreatedAt        string //
	UpdatedAt        string //
	DeletedAt        string //
}

// goodsInfoColumns holds the columns for the table goods_info.
var goodsInfoColumns = GoodsInfoColumns{
	Id:               "id",
	Name:             "name",
	Status:           "status",
	Sort:             "sort",
	PriceCents:       "price_cents",
	Level1CategoryId: "level1_category_id",
	Level2CategoryId: "level2_category_id",
	Level3CategoryId: "level3_category_id",
	Brand:            "brand",
	Tags:             "tags",
	DetailInfo:       "detail_info",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
	DeletedAt:        "deleted_at",
}

// NewGoodsInfoDao creates and returns a new DAO object for table data access.
func NewGoodsInfoDao(handlers ...gdb.ModelHandler) *GoodsInfoDao {
	return &GoodsInfoDao{
		group:    "default",
		table:    "goods_info",
		columns:  goodsInfoColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *GoodsInfoDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *GoodsInfoDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *GoodsInfoDao) Columns() GoodsInfoColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *GoodsInfoDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *GoodsInfoDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *GoodsInfoDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
