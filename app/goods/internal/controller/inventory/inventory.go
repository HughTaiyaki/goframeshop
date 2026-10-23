// Package inventory is the gRPC transport layer for stock reservation.
//
// No public method: the interceptor already demands a token for every RPC
// here. The outcome of a call is part of a successful response; only a broken
// request or a reused operation id is an RPC error.
package inventory

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type Controller struct {
	v1.UnimplementedInventoryServiceServer
	inventory service.IInventory
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterInventoryServiceServer(s.Server, &Controller{inventory: logic.Inventory()})
}

func (c *Controller) ReserveInventory(ctx context.Context, req *v1.ReserveInventoryRequest) (*v1.InventoryOperationResponse, error) {
	result, err := c.inventory.Reserve(ctx, req.OperationId, req.OrderId, toItems(req.Items))
	return respond(result, err)
}

func (c *Controller) CommitInventory(ctx context.Context, req *v1.CommitInventoryRequest) (*v1.InventoryOperationResponse, error) {
	result, err := c.inventory.Commit(ctx, req.OperationId, req.OrderId)
	return respond(result, err)
}

func (c *Controller) ReleaseInventory(ctx context.Context, req *v1.ReleaseInventoryRequest) (*v1.InventoryOperationResponse, error) {
	result, err := c.inventory.Release(ctx, req.OperationId, req.OrderId, toItems(req.Items))
	return respond(result, err)
}

func (c *Controller) QueryInventoryOperation(ctx context.Context, req *v1.QueryInventoryOperationRequest) (*v1.QueryInventoryOperationResponse, error) {
	result, err := c.inventory.Query(ctx, req.OperationId)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	if result == nil {
		return &v1.QueryInventoryOperationResponse{Found: false}, nil
	}
	return &v1.QueryInventoryOperationResponse{Found: true, Operation: toProto(result)}, nil
}

func respond(result *service.OperationResult, err error) (*v1.InventoryOperationResponse, error) {
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return toProto(result), nil
}

func toItems(items []*v1.InventoryItem) []service.InventoryItem {
	out := make([]service.InventoryItem, 0, len(items))
	for _, item := range items {
		out = append(out, service.InventoryItem{GoodsID: item.GetGoodsId(), Quantity: item.GetQuantity()})
	}
	return out
}

var outcomes = map[string]v1.OperationOutcome{
	service.OutcomeSucceeded:         v1.OperationOutcome_SUCCEEDED,
	service.OutcomeInsufficientStock: v1.OperationOutcome_INSUFFICIENT_STOCK,
	service.OutcomeGoodsUnavailable:  v1.OperationOutcome_GOODS_UNAVAILABLE,
	service.OutcomeAlreadyReleased:   v1.OperationOutcome_ALREADY_RELEASED,
	service.OutcomeStateConflict:     v1.OperationOutcome_STATE_CONFLICT,
	service.OutcomeNotReady:          v1.OperationOutcome_NOT_READY,
}

var states = map[string]v1.ReservationState{
	service.StateReserved:  v1.ReservationState_RESERVED,
	service.StateCommitted: v1.ReservationState_COMMITTED,
	service.StateReleased:  v1.ReservationState_RELEASED,
}

func toProto(result *service.OperationResult) *v1.InventoryOperationResponse {
	out := &v1.InventoryOperationResponse{
		OperationId:     result.OperationID,
		OriginalOutcome: outcomes[result.Outcome],
		Reservations:    make([]*v1.ReservationStatus, 0, len(result.Reservations)),
	}
	for _, r := range result.Reservations {
		out.Reservations = append(out.Reservations, &v1.ReservationStatus{
			GoodsId: r.GoodsID, Quantity: r.Quantity, State: states[r.State],
		})
	}
	return out
}
