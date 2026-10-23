// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// CategoryInfo is the golang structure for table category_info.
type CategoryInfo struct {
	Id         uint64      `json:"id"         orm:"id"           description:""`                                      //
	ParentId   uint64      `json:"parentId"   orm:"parent_id"    description:"0 for a top-level category"`            // 0 for a top-level category
	Level      uint        `json:"level"      orm:"level"        description:"1, 2 or 3"`                             // 1, 2 or 3
	Name       string      `json:"name"       orm:"name"         description:""`                                      //
	IconFileId uint64      `json:"iconFileId" orm:"icon_file_id" description:"file service id, 0 for none"`           // file service id, 0 for none
	IconUrl    string      `json:"iconUrl"    orm:"icon_url"     description:"copied from the file service on write"` // copied from the file service on write
	Sort       uint        `json:"sort"       orm:"sort"         description:"ascending"`                             // ascending
	CreatedAt  *gtime.Time `json:"createdAt"  orm:"created_at"   description:""`                                      //
	UpdatedAt  *gtime.Time `json:"updatedAt"  orm:"updated_at"   description:""`                                      //
	DeletedAt  *gtime.Time `json:"deletedAt"  orm:"deleted_at"   description:""`                                      //
}
