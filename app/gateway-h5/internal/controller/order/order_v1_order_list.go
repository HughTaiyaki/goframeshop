package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) OrderList(ctx context.Context, req *v1.OrderListReq) (res *v1.OrderListRes, err error) {
	grpcRes, err := c.OrdersClient.ListMyOrders(ctx, &orderv1.ListMyOrdersRequest{
		Status: orderv1.OrderStatus(orderv1.OrderStatus_value[req.Status]),
		Page:   req.Page,
		Size:   req.Size,
	})
	if err != nil {
		return nil, err
	}
	return toOrderList(grpcRes), nil
}
