// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// CartItem is the golang structure of table cart_item for DAO operations like Where/Data.
type CartItem struct {
	g.Meta    `orm:"table:cart_item, do:true"`
	Id        any         //
	UserId    any         //
	GoodsId   any         //
	Quantity  any         // 1-99, like an order line
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
}
