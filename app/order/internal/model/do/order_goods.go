// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrderGoods is the golang structure of table order_goods for DAO operations like Where/Data.
type OrderGoods struct {
	g.Meta         `orm:"table:order_goods, do:true"`
	Id             any         //
	OrderId        any         //
	GoodsId        any         //
	GoodsName      any         //
	CoverUrl       any         //
	UnitPriceCents any         //
	Quantity       any         //
	SubtotalCents  any         //
	CreatedAt      *gtime.Time //
}
