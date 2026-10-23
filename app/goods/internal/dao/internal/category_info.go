// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CategoryInfoDao is the data access object for the table category_info.
type CategoryInfoDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  CategoryInfoColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// CategoryInfoColumns defines and stores column names for the table category_info.
type CategoryInfoColumns struct {
	Id         string //
	ParentId   string // 0 for a top-level category
	Level      string // 1, 2 or 3
	Name       string //
	IconFileId string // file service id, 0 for none
	IconUrl    string // copied from the file service on write
	Sort       string // ascending
	CreatedAt  string //
	UpdatedAt  string //
	DeletedAt  string //
}

// categoryInfoColumns holds the columns for the table category_info.
var categoryInfoColumns = CategoryInfoColumns{
	Id:         "id",
	ParentId:   "parent_id",
	Level:      "level",
	Name:       "name",
	IconFileId: "icon_file_id",
	IconUrl:    "icon_url",
	Sort:       "sort",
	CreatedAt:  "created_at",
	UpdatedAt:  "updated_at",
	DeletedAt:  "deleted_at",
}

// NewCategoryInfoDao creates and returns a new DAO object for table data access.
func NewCategoryInfoDao(handlers ...gdb.ModelHandler) *CategoryInfoDao {
	return &CategoryInfoDao{
		group:    "default",
		table:    "category_info",
		columns:  categoryInfoColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CategoryInfoDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CategoryInfoDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CategoryInfoDao) Columns() CategoryInfoColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CategoryInfoDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CategoryInfoDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CategoryInfoDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
