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
	g.Meta         `orm:"table:payment, do:true"`
	PaymentId      any         // chosen by the merchant, stable per order
	OrderNo        any         //
	AmountCents    any         // 0 for a tombstone written by Close
	Currency       any         //
	State          any         //
	ExpiresAt      *gtime.Time // the payer cannot pay after this
	ProviderTxId   any         //
	NotifyAttempts any         //
	NextNotifyAt   *gtime.Time //
	PaidAt         *gtime.Time //
	CreatedAt      *gtime.Time //
	UpdatedAt      *gtime.Time //
}
