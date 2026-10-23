// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// GoodsInfo is the golang structure of table goods_info for DAO operations like Where/Data.
type GoodsInfo struct {
	g.Meta           `orm:"table:goods_info, do:true"`
	Id               any         //
	Name             any         //
	Status           any         // 1 on shelf, 2 off shelf
	Sort             any         // hot weight: 0 not hot, larger first
	PriceCents       any         //
	Level1CategoryId any         //
	Level2CategoryId any         //
	Level3CategoryId any         //
	Brand            any         //
	Tags             any         //
	DetailInfo       any         //
	CreatedAt        *gtime.Time //
	UpdatedAt        *gtime.Time //
	DeletedAt        *gtime.Time //
}
