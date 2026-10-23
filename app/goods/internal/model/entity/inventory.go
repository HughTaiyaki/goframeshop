// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Inventory is the golang structure for table inventory.
type Inventory struct {
	GoodsId   uint64      `json:"goodsId"   orm:"goods_id"   description:""` //
	Available uint        `json:"available" orm:"available"  description:""` //
	Reserved  uint        `json:"reserved"  orm:"reserved"   description:""` //
	Sold      uint        `json:"sold"      orm:"sold"       description:""` //
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""` //
}
