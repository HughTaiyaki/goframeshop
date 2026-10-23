// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrderGoodsDao is the data access object for the table order_goods.
type OrderGoodsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  OrderGoodsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// OrderGoodsColumns defines and stores column names for the table order_goods.
type OrderGoodsColumns struct {
	Id             string //
	OrderId        string //
	GoodsId        string //
	GoodsName      string //
	CoverUrl       string //
	UnitPriceCents string //
	Quantity       string //
	SubtotalCents  string //
	CreatedAt      string //
}

// orderGoodsColumns holds the columns for the table order_goods.
var orderGoodsColumns = OrderGoodsColumns{
	Id:             "id",
	OrderId:        "order_id",
	GoodsId:        "goods_id",
	GoodsName:      "goods_name",
	CoverUrl:       "cover_url",
	UnitPriceCents: "unit_price_cents",
	Quantity:       "quantity",
	SubtotalCents:  "subtotal_cents",
	CreatedAt:      "created_at",
}

// NewOrderGoodsDao creates and returns a new DAO object for table data access.
func NewOrderGoodsDao(handlers ...gdb.ModelHandler) *OrderGoodsDao {
	return &OrderGoodsDao{
		group:    "default",
		table:    "order_goods",
		columns:  orderGoodsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrderGoodsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrderGoodsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrderGoodsDao) Columns() OrderGoodsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrderGoodsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrderGoodsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrderGoodsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
