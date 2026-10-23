package order

import (
	"context"

	paymentv1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) OrderPay(ctx context.Context, req *v1.OrderPayReq) (res *v1.OrderPayRes, err error) {
	grpcRes, err := c.PaymentsClient.PayOrder(ctx, &paymentv1.PayOrderRequest{OrderNo: req.OrderNo})
	if err != nil {
		return nil, err
	}
	return &v1.OrderPayRes{PaymentView: &v1.PaymentView{
		PaymentId:   grpcRes.PaymentId,
		State:       grpcRes.State,
		AmountCents: grpcRes.AmountCents,
		PayUrl:      grpcRes.PayUrl,
		ExpiresAt:   utility.TimeFromTimestamp(grpcRes.ExpiresAt),
		Order:       toOrderView(grpcRes.Order),
	}}, nil
}
