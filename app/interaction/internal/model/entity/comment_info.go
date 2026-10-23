// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// CommentInfo is the golang structure for table comment_info.
type CommentInfo struct {
	Id         uint64      `json:"id"         orm:"id"          description:""`                          //
	ParentId   uint64      `json:"parentId"   orm:"parent_id"   description:"0 for a top-level comment"` // 0 for a top-level comment
	UserId     uint64      `json:"userId"     orm:"user_id"     description:""`                          //
	ObjectType uint        `json:"objectType" orm:"object_type" description:"1 goods, 2 article"`        // 1 goods, 2 article
	ObjectId   uint64      `json:"objectId"   orm:"object_id"   description:""`                          //
	Content    string      `json:"content"    orm:"content"     description:""`                          //
	CreatedAt  *gtime.Time `json:"createdAt"  orm:"created_at"  description:""`                          //
	UpdatedAt  *gtime.Time `json:"updatedAt"  orm:"updated_at"  description:""`                          //
	DeletedAt  *gtime.Time `json:"deletedAt"  orm:"deleted_at"  description:""`                          //
}
