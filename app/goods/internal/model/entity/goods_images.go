// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// GoodsImages is the golang structure for table goods_images.
type GoodsImages struct {
	Id        uint64      `json:"id"        orm:"id"         description:""`          //
	GoodsId   uint64      `json:"goodsId"   orm:"goods_id"   description:""`          //
	FileId    uint64      `json:"fileId"    orm:"file_id"    description:""`          //
	Url       string      `json:"url"       orm:"url"        description:""`          //
	Sort      uint        `json:"sort"      orm:"sort"       description:"ascending"` // ascending
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""`          //
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""`          //
}
