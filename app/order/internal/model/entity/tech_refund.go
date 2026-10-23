// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TechRefund is the golang structure for table tech_refund.
type TechRefund struct {
	RefundId    string      `json:"refundId"    orm:"refund_id"    description:""`                              //
	PaymentId   string      `json:"paymentId"   orm:"payment_id"   description:""`                              //
	OrderId     uint64      `json:"orderId"     orm:"order_id"     description:""`                              //
	AmountCents uint64      `json:"amountCents" orm:"amount_cents" description:""`                              //
	Reason      string      `json:"reason"      orm:"reason"       description:"LATE_PAYMENT or COMMIT_FAILED"` // LATE_PAYMENT or COMMIT_FAILED
	State       string      `json:"state"       orm:"state"        description:"PENDING or SUCCEEDED"`          // PENDING or SUCCEEDED
	Attempts    uint        `json:"attempts"    orm:"attempts"     description:""`                              //
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"   description:""`                              //
	UpdatedAt   *gtime.Time `json:"updatedAt"   orm:"updated_at"   description:""`                              //
}
