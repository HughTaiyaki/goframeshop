// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PositionInfo is the golang structure for table position_info.
type PositionInfo struct {
	Id           uint64      `json:"id"           orm:"id"             description:""`                                          //
	FileId       uint64      `json:"fileId"       orm:"file_id"        description:""`                                          //
	PicUrl       string      `json:"picUrl"       orm:"pic_url"        description:"copied from the file service on write"`     // copied from the file service on write
	GoodsId      uint64      `json:"goodsId"      orm:"goods_id"       description:""`                                          //
	GoodsName    string      `json:"goodsName"    orm:"goods_name"     description:"copied from the goods service on write"`    // copied from the goods service on write
	GoodsOnShelf uint        `json:"goodsOnShelf" orm:"goods_on_shelf" description:"0 once the goods is off shelf or deleted"`  // 0 once the goods is off shelf or deleted
	GoodsVersion uint64      `json:"goodsVersion" orm:"goods_version"  description:"version of the last goods.changed applied"` // version of the last goods.changed applied
	Link         string      `json:"link"         orm:"link"           description:"http(s) only; empty means the goods page"`  // http(s) only; empty means the goods page
	Sort         uint        `json:"sort"         orm:"sort"           description:"ascending"`                                 // ascending
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"     description:""`                                          //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"     description:""`                                          //
	DeletedAt    *gtime.Time `json:"deletedAt"    orm:"deleted_at"     description:""`                                          //
}
