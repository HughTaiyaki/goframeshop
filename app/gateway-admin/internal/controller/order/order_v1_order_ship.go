package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func (c *ControllerV1) OrderShip(ctx context.Context, req *v1.OrderShipReq) (res *v1.OrderShipRes, err error) {
	grpcRes, err := c.OrdersClient.ShipOrder(ctx, &orderv1.ShipOrderRequest{
		OrderNo: req.OrderNo, Carrier: req.Carrier, TrackingNo: req.TrackingNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.OrderShipRes{OrderView: toOrderView(grpcRes.Order)}, nil
}
