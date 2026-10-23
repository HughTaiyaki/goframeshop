// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// CategoryInfo is the golang structure of table category_info for DAO operations like Where/Data.
type CategoryInfo struct {
	g.Meta     `orm:"table:category_info, do:true"`
	Id         any         //
	ParentId   any         // 0 for a top-level category
	Level      any         // 1, 2 or 3
	Name       any         //
	IconFileId any         // file service id, 0 for none
	IconUrl    any         // copied from the file service on write
	Sort       any         // ascending
	CreatedAt  *gtime.Time //
	UpdatedAt  *gtime.Time //
	DeletedAt  *gtime.Time //
}
