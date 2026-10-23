// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ConsigneeInfo is the golang structure for table consignee_info.
type ConsigneeInfo struct {
	Id        uint64      `json:"id"        orm:"id"         description:""` //
	UserId    uint64      `json:"userId"    orm:"user_id"    description:""` //
	IsDefault uint        `json:"isDefault" orm:"is_default" description:""` //
	Name      string      `json:"name"      orm:"name"       description:""` //
	Phone     string      `json:"phone"     orm:"phone"      description:""` //
	Province  string      `json:"province"  orm:"province"   description:""` //
	City      string      `json:"city"      orm:"city"       description:""` //
	Town      string      `json:"town"      orm:"town"       description:""` //
	Street    string      `json:"street"    orm:"street"     description:""` //
	Detail    string      `json:"detail"    orm:"detail"     description:""` //
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""` //
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""` //
	DeletedAt *gtime.Time `json:"deletedAt" orm:"deleted_at" description:""` //
}
