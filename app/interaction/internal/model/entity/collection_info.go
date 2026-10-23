// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// CollectionInfo is the golang structure for table collection_info.
type CollectionInfo struct {
	Id         uint64      `json:"id"         orm:"id"          description:""`                   //
	UserId     uint64      `json:"userId"     orm:"user_id"     description:""`                   //
	ObjectType uint        `json:"objectType" orm:"object_type" description:"1 goods, 2 article"` // 1 goods, 2 article
	ObjectId   uint64      `json:"objectId"   orm:"object_id"   description:""`                   //
	CreatedAt  *gtime.Time `json:"createdAt"  orm:"created_at"  description:""`                   //
	UpdatedAt  *gtime.Time `json:"updatedAt"  orm:"updated_at"  description:""`                   //
}
