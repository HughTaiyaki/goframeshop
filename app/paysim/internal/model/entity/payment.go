// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Payment is the golang structure for table payment.
type Payment struct {
	PaymentId      string      `json:"paymentId"      orm:"payment_id"      description:"chosen by the merchant, stable per order"` // chosen by the merchant, stable per order
	OrderNo        string      `json:"orderNo"        orm:"order_no"        description:""`                                         //
	AmountCents    uint64      `json:"amountCents"    orm:"amount_cents"    description:"0 for a tombstone written by Close"`       // 0 for a tombstone written by Close
	Currency       string      `json:"currency"       orm:"currency"        description:""`                                         //
	State          string      `json:"state"          orm:"state"           description:""`                                         //
	ExpiresAt      *gtime.Time `json:"expiresAt"      orm:"expires_at"      description:"the payer cannot pay after this"`          // the payer cannot pay after this
	ProviderTxId   string      `json:"providerTxId"   orm:"provider_tx_id"  description:""`                                         //
	NotifyAttempts uint        `json:"notifyAttempts" orm:"notify_attempts" description:""`                                         //
	NextNotifyAt   *gtime.Time `json:"nextNotifyAt"   orm:"next_notify_at"  description:""`                                         //
	PaidAt         *gtime.Time `json:"paidAt"         orm:"paid_at"         description:""`                                         //
	CreatedAt      *gtime.Time `json:"createdAt"      orm:"created_at"      description:""`                                         //
	UpdatedAt      *gtime.Time `json:"updatedAt"      orm:"updated_at"      description:""`                                         //
}
