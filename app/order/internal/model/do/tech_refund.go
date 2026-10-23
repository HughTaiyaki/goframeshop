// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TechRefund is the golang structure of table tech_refund for DAO operations like Where/Data.
type TechRefund struct {
	g.Meta      `orm:"table:tech_refund, do:true"`
	RefundId    any         //
	PaymentId   any         //
	OrderId     any         //
	AmountCents any         //
	Reason      any         // LATE_PAYMENT or COMMIT_FAILED
	State       any         // PENDING or SUCCEEDED
	Attempts    any         //
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
}
