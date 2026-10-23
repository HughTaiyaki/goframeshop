// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Payment is the golang structure for table payment.
type Payment struct {
	PaymentId    string      `json:"paymentId"    orm:"payment_id"     description:""` //
	OrderId      uint64      `json:"orderId"      orm:"order_id"       description:""` //
	AmountCents  uint64      `json:"amountCents"  orm:"amount_cents"   description:""` //
	State        string      `json:"state"        orm:"state"          description:""` //
	ProviderTxId string      `json:"providerTxId" orm:"provider_tx_id" description:""` //
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"     description:""` //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"     description:""` //
}
