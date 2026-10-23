// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Reservation is the golang structure for table reservation.
type Reservation struct {
	Id        uint64      `json:"id"        orm:"id"         description:""`                                //
	OrderId   string      `json:"orderId"   orm:"order_id"   description:""`                                //
	GoodsId   uint64      `json:"goodsId"   orm:"goods_id"   description:""`                                //
	Quantity  uint        `json:"quantity"  orm:"quantity"   description:""`                                //
	State     string      `json:"state"     orm:"state"      description:"RESERVED, COMMITTED or RELEASED"` // RESERVED, COMMITTED or RELEASED
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""`                                //
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""`                                //
}
