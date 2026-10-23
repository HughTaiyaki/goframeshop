package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func (c *ControllerV1) OrderList(ctx context.Context, req *v1.OrderListReq) (res *v1.OrderListRes, err error) {
	grpcReq := &orderv1.ListOrdersRequest{
		OrderNo: req.OrderNo,
		UserId:  req.UserId,
		Status:  orderv1.OrderStatus(orderv1.OrderStatus_value[req.Status]),
		Page:    req.Page,
		Size:    req.Size,
	}
	if req.CreatedFrom != nil {
		grpcReq.CreatedFrom = timestamppb.New(req.CreatedFrom.Time)
	}
	if req.CreatedTo != nil {
		grpcReq.CreatedTo = timestamppb.New(req.CreatedTo.Time)
	}
	grpcRes, err := c.OrdersClient.ListOrders(ctx, grpcReq)
	if err != nil {
		return nil, err
	}
	return toOrderList(grpcRes), nil
}
