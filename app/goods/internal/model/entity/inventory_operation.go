// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// InventoryOperation is the golang structure for table inventory_operation.
type InventoryOperation struct {
	OperationId string      `json:"operationId" orm:"operation_id" description:""`                                //
	Kind        string      `json:"kind"        orm:"kind"         description:"RESERVE, COMMIT or RELEASE"`      // RESERVE, COMMIT or RELEASE
	OrderId     string      `json:"orderId"     orm:"order_id"     description:""`                                //
	RequestHash string      `json:"requestHash" orm:"request_hash" description:"sha256 of the canonical request"` // sha256 of the canonical request
	Outcome     string      `json:"outcome"     orm:"outcome"      description:""`                                //
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"   description:""`                                //
}
