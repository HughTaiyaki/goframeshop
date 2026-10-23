// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Inventory is the golang structure of table inventory for DAO operations like Where/Data.
type Inventory struct {
	g.Meta    `orm:"table:inventory, do:true"`
	GoodsId   any         //
	Available any         //
	Reserved  any         //
	Sold      any         //
	UpdatedAt *gtime.Time //
}
