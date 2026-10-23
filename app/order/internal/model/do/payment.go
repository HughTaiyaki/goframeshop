// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Payment is the golang structure of table payment for DAO operations like Where/Data.
type Payment struct {
	g.Meta       `orm:"table:payment, do:true"`
	PaymentId    any         //
	OrderId      any         //
	AmountCents  any         //
	State        any         //
	ProviderTxId any         //
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}
