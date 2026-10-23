// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// FileInfo is the golang structure for table file_info.
type FileInfo struct {
	Id          uint64      `json:"id"          orm:"id"           description:""`                                     //
	UploaderId  uint64      `json:"uploaderId"  orm:"uploader_id"  description:"admin_info.id of the uploader"`        // admin_info.id of the uploader
	Name        string      `json:"name"        orm:"name"         description:"original file name, for display only"` // original file name, for display only
	Storage     string      `json:"storage"     orm:"storage"      description:"qiniu or local"`                       // qiniu or local
	ObjectKey   string      `json:"objectKey"   orm:"object_key"   description:"key inside the storage backend"`       // key inside the storage backend
	Url         string      `json:"url"         orm:"url"          description:""`                                     //
	Size        uint64      `json:"size"        orm:"size"         description:""`                                     //
	ContentType string      `json:"contentType" orm:"content_type" description:""`                                     //
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"   description:""`                                     //
	UpdatedAt   *gtime.Time `json:"updatedAt"   orm:"updated_at"   description:""`                                     //
	DeletedAt   *gtime.Time `json:"deletedAt"   orm:"deleted_at"   description:""`                                     //
}
