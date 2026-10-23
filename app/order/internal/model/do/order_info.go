// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrderInfo is the golang structure of table order_info for DAO operations like Where/Data.
type OrderInfo struct {
	g.Meta           `orm:"table:order_info, do:true"`
	Id               any         //
	OrderNo          any         // public order number; also the inventory order id
	UserId           any         //
	RequestId        any         //
	RequestHash      any         // sha256 of the normalised request
	Status           any         //
	CloseReason      any         //
	TotalCents       any         // computed by the server from goods prices
	ConsigneeName    any         //
	ConsigneePhone   any         //
	ConsigneeAddress any         //
	Remark           any         //
	ReserveDeadline  *gtime.Time //
	ExpiresAt        *gtime.Time //
	ClosedAt         *gtime.Time //
	PaidAt           *gtime.Time //
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
}
