// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// UserInfo is the golang structure for table user_info.
type UserInfo struct {
	Id               uint64      `json:"id"               orm:"id"                 description:""`                            //
	Name             string      `json:"name"             orm:"name"               description:""`                            //
	Avatar           string      `json:"avatar"           orm:"avatar"             description:""`                            //
	PasswordHash     string      `json:"passwordHash"     orm:"password_hash"      description:""`                            //
	PasswordSalt     []byte      `json:"passwordSalt"     orm:"password_salt"      description:""`                            //
	Sex              uint        `json:"sex"              orm:"sex"                description:"0 unknown, 1 male, 2 female"` // 0 unknown, 1 male, 2 female
	Status           uint        `json:"status"           orm:"status"             description:"1 active, 2 blocked"`         // 1 active, 2 blocked
	Sign             string      `json:"sign"             orm:"sign"               description:""`                            //
	SecretAnswerHash string      `json:"secretAnswerHash" orm:"secret_answer_hash" description:""`                            //
	CreatedAt        *gtime.Time `json:"createdAt"        orm:"created_at"         description:""`                            //
	UpdatedAt        *gtime.Time `json:"updatedAt"        orm:"updated_at"         description:""`                            //
	DeletedAt        *gtime.Time `json:"deletedAt"        orm:"deleted_at"         description:""`                            //
}
