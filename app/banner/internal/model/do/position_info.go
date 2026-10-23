// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PositionInfo is the golang structure of table position_info for DAO operations like Where/Data.
type PositionInfo struct {
	g.Meta       `orm:"table:position_info, do:true"`
	Id           any         //
	FileId       any         //
	PicUrl       any         // copied from the file service on write
	GoodsId      any         //
	GoodsName    any         // copied from the goods service on write
	GoodsOnShelf any         // 0 once the goods is off shelf or deleted
	GoodsVersion any         // version of the last goods.changed applied
	Link         any         // http(s) only; empty means the goods page
	Sort         any         // ascending
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
	DeletedAt    *gtime.Time //
}
