package order

import (
	"context"

	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

func (c *ControllerV1) RefundDetail(ctx context.Context, req *v1.RefundDetailReq) (res *v1.RefundDetailRes, err error) {
	grpcRes, err := c.RefundsClient.GetMyRefund(ctx, &refundv1.GetMyRefundRequest{RefundNo: req.RefundNo})
	if err != nil {
		return nil, err
	}
	return &v1.RefundDetailRes{RefundView: toRefundView(grpcRes.Refund)}, nil
}
