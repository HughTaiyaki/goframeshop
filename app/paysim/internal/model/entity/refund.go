// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Refund is the golang structure for table refund.
type Refund struct {
	RefundId    string      `json:"refundId"    orm:"refund_id"    description:""` //
	PaymentId   string      `json:"paymentId"   orm:"payment_id"   description:""` //
	AmountCents uint64      `json:"amountCents" orm:"amount_cents" description:""` //
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"   description:""` //
}
