// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OrderInfoDao is the data access object for the table order_info.
type OrderInfoDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  OrderInfoColumns   // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// OrderInfoColumns defines and stores column names for the table order_info.
type OrderInfoColumns struct {
	Id               string //
	OrderNo          string // public order number; also the inventory order id
	UserId           string //
	RequestId        string //
	RequestHash      string // sha256 of the normalised request
	Status           string //
	CloseReason      string //
	TotalCents       string // computed by the server from goods prices
	ConsigneeName    string //
	ConsigneePhone   string //
	ConsigneeAddress string //
	Remark           string //
	FromCart         string // 1: checked out from the cart
	ReserveDeadline  string //
	ExpiresAt        string //
	ClosedAt         string //
	PaidAt           string //
	Carrier          string // shipping company
	TrackingNo       string // waybill number
	ShippedAt        string //
	ConfirmDeadline  string //
	CompletedAt      string //
	CompleteReason   string // USER_CONFIRMED or AUTO_CONFIRMED
	CreatedAt        string //
	UpdatedAt        string //
}

// orderInfoColumns holds the columns for the table order_info.
var orderInfoColumns = OrderInfoColumns{
	Id:               "id",
	OrderNo:          "order_no",
	UserId:           "user_id",
	RequestId:        "request_id",
	RequestHash:      "request_hash",
	Status:           "status",
	CloseReason:      "close_reason",
	TotalCents:       "total_cents",
	ConsigneeName:    "consignee_name",
	ConsigneePhone:   "consignee_phone",
	ConsigneeAddress: "consignee_address",
	Remark:           "remark",
	FromCart:         "from_cart",
	ReserveDeadline:  "reserve_deadline",
	ExpiresAt:        "expires_at",
	ClosedAt:         "closed_at",
	PaidAt:           "paid_at",
	Carrier:          "carrier",
	TrackingNo:       "tracking_no",
	ShippedAt:        "shipped_at",
	ConfirmDeadline:  "confirm_deadline",
	CompletedAt:      "completed_at",
	CompleteReason:   "complete_reason",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewOrderInfoDao creates and returns a new DAO object for table data access.
func NewOrderInfoDao(handlers ...gdb.ModelHandler) *OrderInfoDao {
	return &OrderInfoDao{
		group:    "default",
		table:    "order_info",
		columns:  orderInfoColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OrderInfoDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OrderInfoDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OrderInfoDao) Columns() OrderInfoColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OrderInfoDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OrderInfoDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OrderInfoDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
