// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Refund is the golang structure of table refund for DAO operations like Where/Data.
type Refund struct {
	g.Meta      `orm:"table:refund, do:true"`
	RefundId    any         //
	PaymentId   any         //
	AmountCents any         //
	CreatedAt   *gtime.Time //
}
