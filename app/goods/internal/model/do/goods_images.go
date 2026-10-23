// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// GoodsImages is the golang structure of table goods_images for DAO operations like Where/Data.
type GoodsImages struct {
	g.Meta    `orm:"table:goods_images, do:true"`
	Id        any         //
	GoodsId   any         //
	FileId    any         //
	Url       any         //
	Sort      any         // ascending
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
}
