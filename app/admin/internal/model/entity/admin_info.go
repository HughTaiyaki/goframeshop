// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AdminInfo is the golang structure for table admin_info.
type AdminInfo struct {
	Id           uint64      `json:"id"           orm:"id"            description:""`                            //
	Name         string      `json:"name"         orm:"name"          description:""`                            //
	PasswordHash string      `json:"passwordHash" orm:"password_hash" description:""`                            //
	PasswordSalt []byte      `json:"passwordSalt" orm:"password_salt" description:""`                            //
	IsSuper      uint        `json:"isSuper"      orm:"is_super"      description:"1 may create administrators"` // 1 may create administrators
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    description:""`                            //
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"    description:""`                            //
	DeletedAt    *gtime.Time `json:"deletedAt"    orm:"deleted_at"    description:""`                            //
}
