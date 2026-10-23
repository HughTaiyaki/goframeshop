// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// InventoryOperationDao is the data access object for the table inventory_operation.
type InventoryOperationDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  InventoryOperationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// InventoryOperationColumns defines and stores column names for the table inventory_operation.
type InventoryOperationColumns struct {
	OperationId string //
	Kind        string // RESERVE, COMMIT or RELEASE
	OrderId     string //
	RequestHash string // sha256 of the canonical request
	Outcome     string //
	CreatedAt   string //
}

// inventoryOperationColumns holds the columns for the table inventory_operation.
var inventoryOperationColumns = InventoryOperationColumns{
	OperationId: "operation_id",
	Kind:        "kind",
	OrderId:     "order_id",
	RequestHash: "request_hash",
	Outcome:     "outcome",
	CreatedAt:   "created_at",
}

// NewInventoryOperationDao creates and returns a new DAO object for table data access.
func NewInventoryOperationDao(handlers ...gdb.ModelHandler) *InventoryOperationDao {
	return &InventoryOperationDao{
		group:    "default",
		table:    "inventory_operation",
		columns:  inventoryOperationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *InventoryOperationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *InventoryOperationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *InventoryOperationDao) Columns() InventoryOperationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *InventoryOperationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *InventoryOperationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *InventoryOperationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
