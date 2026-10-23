// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OrderInfo is the golang structure for table order_info.
type OrderInfo struct {
	Id               uint64      `json:"id"               orm:"id"                description:""`                                                 //
	OrderNo          string      `json:"orderNo"          orm:"order_no"          description:"public order number; also the inventory order id"` // public order number; also the inventory order id
	UserId           uint64      `json:"userId"           orm:"user_id"           description:""`                                                 //
	RequestId        string      `json:"requestId"        orm:"request_id"        description:""`                                                 //
	RequestHash      string      `json:"requestHash"      orm:"request_hash"      description:"sha256 of the normalised request"`                 // sha256 of the normalised request
	Status           string      `json:"status"           orm:"status"            description:""`                                                 //
	CloseReason      string      `json:"closeReason"      orm:"close_reason"      description:""`                                                 //
	TotalCents       uint64      `json:"totalCents"       orm:"total_cents"       description:"computed by the server from goods prices"`         // computed by the server from goods prices
	ConsigneeName    string      `json:"consigneeName"    orm:"consignee_name"    description:""`                                                 //
	ConsigneePhone   string      `json:"consigneePhone"   orm:"consignee_phone"   description:""`                                                 //
	ConsigneeAddress string      `json:"consigneeAddress" orm:"consignee_address" description:""`                                                 //
	Remark           string      `json:"remark"           orm:"remark"            description:""`                                                 //
	FromCart         uint        `json:"fromCart"         orm:"from_cart"         description:"1: checked out from the cart"`                     // 1: checked out from the cart
	ReserveDeadline  *gtime.Time `json:"reserveDeadline"  orm:"reserve_deadline"  description:""`                                                 //
	ExpiresAt        *gtime.Time `json:"expiresAt"        orm:"expires_at"        description:""`                                                 //
	ClosedAt         *gtime.Time `json:"closedAt"         orm:"closed_at"         description:""`                                                 //
	PaidAt           *gtime.Time `json:"paidAt"           orm:"paid_at"           description:""`                                                 //
	CreatedAt        *gtime.Time `json:"createdAt"        orm:"created_at"        description:""`                                                 //
	UpdatedAt        *gtime.Time `json:"updatedAt"        orm:"updated_at"        description:""`                                                 //
}
