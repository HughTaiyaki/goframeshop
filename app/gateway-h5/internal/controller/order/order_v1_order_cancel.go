package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) OrderCancel(ctx context.Context, req *v1.OrderCancelReq) (res *v1.OrderCancelRes, err error) {
	grpcRes, err := c.OrdersClient.CancelOrder(ctx, &orderv1.CancelOrderRequest{OrderNo: req.OrderNo})
	if err != nil {
		return nil, err
	}
	return &v1.OrderCancelRes{OrderView: toOrderView(grpcRes.Order)}, nil
}
