package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) OrderCreate(ctx context.Context, req *v1.OrderCreateReq) (res *v1.OrderCreateRes, err error) {
	grpcReq := &orderv1.CreateOrderRequest{
		RequestId: req.RequestId, ConsigneeId: req.ConsigneeId, Remark: req.Remark, FromCart: req.FromCart,
	}
	for _, item := range req.Items {
		grpcReq.Items = append(grpcReq.Items, &orderv1.OrderItemInput{GoodsId: item.GoodsId, Quantity: item.Quantity})
	}
	grpcRes, err := c.OrdersClient.CreateOrder(ctx, grpcReq)
	if err != nil {
		return nil, err
	}
	return &v1.OrderCreateRes{OrderView: toOrderView(grpcRes.Order)}, nil
}
