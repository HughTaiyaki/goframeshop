// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// RefundInfo is the golang structure of table refund_info for DAO operations like Where/Data.
type RefundInfo struct {
	g.Meta      `orm:"table:refund_info, do:true"`
	Id          any         //
	RefundNo    any         //
	OrderId     any         //
	UserId      any         //
	GoodsId     any         //
	Reason      any         //
	AmountCents any         // the goods line subtotal
	Status      any         // PENDING, APPROVED or REJECTED
	ReviewNote  any         // why it was approved or rejected
	ReviewedAt  *gtime.Time //
	RefundedAt  *gtime.Time //
	Attempts    any         //
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
}
