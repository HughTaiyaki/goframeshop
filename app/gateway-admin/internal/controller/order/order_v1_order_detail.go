package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func (c *ControllerV1) OrderDetail(ctx context.Context, req *v1.OrderDetailReq) (res *v1.OrderDetailRes, err error) {
	grpcRes, err := c.OrdersClient.GetOrder(ctx, &orderv1.GetOrderRequest{OrderNo: req.OrderNo})
	if err != nil {
		return nil, err
	}
	return &v1.OrderDetailRes{OrderView: toOrderView(grpcRes.Order)}, nil
}
