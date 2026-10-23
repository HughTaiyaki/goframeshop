package order

import (
	"context"

	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) RefundCreate(ctx context.Context, req *v1.RefundCreateReq) (res *v1.RefundCreateRes, err error) {
	grpcRes, err := c.RefundsClient.CreateRefund(ctx, &refundv1.CreateRefundRequest{
		OrderNo: req.OrderNo, GoodsId: req.GoodsId, Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RefundCreateRes{RefundView: toRefundView(grpcRes.Refund)}, nil
}
