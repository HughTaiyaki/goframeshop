package order

import (
	"context"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) OrderConfirm(ctx context.Context, req *v1.OrderConfirmReq) (res *v1.OrderConfirmRes, err error) {
	grpcRes, err := c.OrdersClient.ConfirmReceipt(ctx, &orderv1.ConfirmReceiptRequest{OrderNo: req.OrderNo})
	if err != nil {
		return nil, err
	}
	return &v1.OrderConfirmRes{OrderView: toOrderView(grpcRes.Order)}, nil
}
