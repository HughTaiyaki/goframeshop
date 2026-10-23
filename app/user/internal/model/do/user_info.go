// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// UserInfo is the golang structure of table user_info for DAO operations like Where/Data.
type UserInfo struct {
	g.Meta           `orm:"table:user_info, do:true"`
	Id               any         //
	Name             any         //
	Avatar           any         //
	PasswordHash     any         //
	PasswordSalt     []byte      //
	Sex              any         // 0 unknown, 1 male, 2 female
	Status           any         // 1 active, 2 blocked
	Sign             any         //
	SecretAnswerHash any         //
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
	DeletedAt        *gtime.Time //
}
