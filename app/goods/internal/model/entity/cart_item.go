// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// CartItem is the golang structure for table cart_item.
type CartItem struct {
	Id        uint64      `json:"id"        orm:"id"         description:""`                         //
	UserId    uint64      `json:"userId"    orm:"user_id"    description:""`                         //
	GoodsId   uint64      `json:"goodsId"   orm:"goods_id"   description:""`                         //
	Quantity  uint        `json:"quantity"  orm:"quantity"   description:"1-99, like an order line"` // 1-99, like an order line
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""`                         //
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""`                         //
}
