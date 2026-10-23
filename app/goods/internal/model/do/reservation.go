// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Reservation is the golang structure of table reservation for DAO operations like Where/Data.
type Reservation struct {
	g.Meta    `orm:"table:reservation, do:true"`
	Id        any         //
	OrderId   any         //
	GoodsId   any         //
	Quantity  any         //
	State     any         // RESERVED, COMMITTED or RELEASED
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
}
