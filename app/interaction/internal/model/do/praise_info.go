// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PraiseInfo is the golang structure of table praise_info for DAO operations like Where/Data.
type PraiseInfo struct {
	g.Meta     `orm:"table:praise_info, do:true"`
	Id         any         //
	UserId     any         //
	ObjectType any         // 1 goods, 2 article
	ObjectId   any         //
	CreatedAt  *gtime.Time //
	UpdatedAt  *gtime.Time //
}
