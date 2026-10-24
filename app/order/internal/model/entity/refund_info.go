// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// RefundInfo is the golang structure for table refund_info.
type RefundInfo struct {
	Id          uint64      `json:"id"          orm:"id"           description:""`                                //
	RefundNo    string      `json:"refundNo"    orm:"refund_no"    description:""`                                //
	OrderId     uint64      `json:"orderId"     orm:"order_id"     description:""`                                //
	UserId      uint64      `json:"userId"      orm:"user_id"      description:""`                                //
	GoodsId     uint64      `json:"goodsId"     orm:"goods_id"     description:""`                                //
	Reason      string      `json:"reason"      orm:"reason"       description:""`                                //
	AmountCents uint64      `json:"amountCents" orm:"amount_cents" description:"the goods line subtotal"`         // the goods line subtotal
	Status      string      `json:"status"      orm:"status"       description:"PENDING, APPROVED or REJECTED"`   // PENDING, APPROVED or REJECTED
	ReviewNote  string      `json:"reviewNote"  orm:"review_note"  description:"why it was approved or rejected"` // why it was approved or rejected
	ReviewedAt  *gtime.Time `json:"reviewedAt"  orm:"reviewed_at"  description:""`                                //
	RefundedAt  *gtime.Time `json:"refundedAt"  orm:"refunded_at"  description:""`                                //
	Attempts    uint        `json:"attempts"    orm:"attempts"     description:""`                                //
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"   description:""`                                //
	UpdatedAt   *gtime.Time `json:"updatedAt"   orm:"updated_at"   description:""`                                //
}
