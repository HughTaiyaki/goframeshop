// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// InventoryDao is the data access object for the table inventory.
type InventoryDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  InventoryColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// InventoryColumns defines and stores column names for the table inventory.
type InventoryColumns struct {
	GoodsId   string //
	Available string //
	Reserved  string //
	Sold      string //
	UpdatedAt string //
}

// inventoryColumns holds the columns for the table inventory.
var inventoryColumns = InventoryColumns{
	GoodsId:   "goods_id",
	Available: "available",
	Reserved:  "reserved",
	Sold:      "sold",
	UpdatedAt: "updated_at",
}

// NewInventoryDao creates and returns a new DAO object for table data access.
func NewInventoryDao(handlers ...gdb.ModelHandler) *InventoryDao {
	return &InventoryDao{
		group:    "default",
		table:    "inventory",
		columns:  inventoryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *InventoryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *InventoryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *InventoryDao) Columns() InventoryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *InventoryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *InventoryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *InventoryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
