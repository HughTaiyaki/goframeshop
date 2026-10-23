// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// FileInfo is the golang structure of table file_info for DAO operations like Where/Data.
type FileInfo struct {
	g.Meta      `orm:"table:file_info, do:true"`
	Id          any         //
	UploaderId  any         // admin_info.id of the uploader
	Name        any         // original file name, for display only
	Storage     any         // qiniu or local
	ObjectKey   any         // key inside the storage backend
	Url         any         //
	Size        any         //
	ContentType any         //
	CreatedAt   *gtime.Time //
	UpdatedAt   *gtime.Time //
	DeletedAt   *gtime.Time //
}
