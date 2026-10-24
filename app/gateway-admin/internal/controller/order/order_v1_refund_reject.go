package order

import (
	"context"

	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func (c *ControllerV1) RefundReject(ctx context.Context, req *v1.RefundRejectReq) (res *v1.RefundRejectRes, err error) {
	grpcRes, err := c.RefundsClient.RejectRefund(ctx, &refundv1.RejectRefundRequest{RefundNo: req.RefundNo, Note: req.Note})
	if err != nil {
		return nil, err
	}
	return &v1.RefundRejectRes{RefundView: toRefundView(grpcRes.Refund)}, nil
}
