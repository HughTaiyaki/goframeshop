// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// RotationInfo is the golang structure of table rotation_info for DAO operations like Where/Data.
type RotationInfo struct {
	g.Meta    `orm:"table:rotation_info, do:true"`
	Id        any         //
	FileId    any         //
	PicUrl    any         // copied from the file service on write
	Link      any         // http(s) only; empty for no link
	Sort      any         // ascending
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
	DeletedAt *gtime.Time //
}
