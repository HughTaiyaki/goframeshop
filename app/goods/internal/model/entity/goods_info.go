// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// GoodsInfo is the golang structure for table goods_info.
type GoodsInfo struct {
	Id               uint64      `json:"id"               orm:"id"                 description:""`                                    //
	Name             string      `json:"name"             orm:"name"               description:""`                                    //
	Status           uint        `json:"status"           orm:"status"             description:"1 on shelf, 2 off shelf"`             // 1 on shelf, 2 off shelf
	Sort             uint        `json:"sort"             orm:"sort"               description:"hot weight: 0 not hot, larger first"` // hot weight: 0 not hot, larger first
	PriceCents       uint64      `json:"priceCents"       orm:"price_cents"        description:""`                                    //
	Level1CategoryId uint64      `json:"level1CategoryId" orm:"level1_category_id" description:""`                                    //
	Level2CategoryId uint64      `json:"level2CategoryId" orm:"level2_category_id" description:""`                                    //
	Level3CategoryId uint64      `json:"level3CategoryId" orm:"level3_category_id" description:""`                                    //
	Brand            string      `json:"brand"            orm:"brand"              description:""`                                    //
	Tags             string      `json:"tags"             orm:"tags"               description:""`                                    //
	DetailInfo       string      `json:"detailInfo"       orm:"detail_info"        description:""`                                    //
	CreatedAt        *gtime.Time `json:"createdAt"        orm:"created_at"         description:""`                                    //
	UpdatedAt        *gtime.Time `json:"updatedAt"        orm:"updated_at"         description:""`                                    //
	DeletedAt        *gtime.Time `json:"deletedAt"        orm:"deleted_at"         description:""`                                    //
}
