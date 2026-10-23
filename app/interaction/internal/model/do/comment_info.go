// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// CommentInfo is the golang structure of table comment_info for DAO operations like Where/Data.
type CommentInfo struct {
	g.Meta     `orm:"table:comment_info, do:true"`
	Id         any         //
	ParentId   any         // 0 for a top-level comment
	UserId     any         //
	ObjectType any         // 1 goods, 2 article
	ObjectId   any         //
	Content    any         //
	CreatedAt  *gtime.Time //
	UpdatedAt  *gtime.Time //
	DeletedAt  *gtime.Time //
}
