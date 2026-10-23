// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// InventoryOperation is the golang structure of table inventory_operation for DAO operations like Where/Data.
type InventoryOperation struct {
	g.Meta      `orm:"table:inventory_operation, do:true"`
	OperationId any         //
	Kind        any         // RESERVE, COMMIT or RELEASE
	OrderId     any         //
	RequestHash any         // sha256 of the canonical request
	Outcome     any         //
	CreatedAt   *gtime.Time //
}
