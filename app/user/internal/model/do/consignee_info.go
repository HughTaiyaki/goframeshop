// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ConsigneeInfo is the golang structure of table consignee_info for DAO operations like Where/Data.
type ConsigneeInfo struct {
	g.Meta    `orm:"table:consignee_info, do:true"`
	Id        any         //
	UserId    any         //
	IsDefault any         //
	Name      any         //
	Phone     any         //
	Province  any         //
	City      any         //
	Town      any         //
	Street    any         //
	Detail    any         //
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
	DeletedAt *gtime.Time //
}
