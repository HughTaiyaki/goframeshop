// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// RotationInfo is the golang structure for table rotation_info.
type RotationInfo struct {
	Id        uint64      `json:"id"        orm:"id"         description:""`                                      //
	FileId    uint64      `json:"fileId"    orm:"file_id"    description:""`                                      //
	PicUrl    string      `json:"picUrl"    orm:"pic_url"    description:"copied from the file service on write"` // copied from the file service on write
	Link      string      `json:"link"      orm:"link"       description:"http(s) only; empty for no link"`       // http(s) only; empty for no link
	Sort      uint        `json:"sort"      orm:"sort"       description:"ascending"`                             // ascending
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""`                                      //
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""`                                      //
	DeletedAt *gtime.Time `json:"deletedAt" orm:"deleted_at" description:""`                                      //
}
