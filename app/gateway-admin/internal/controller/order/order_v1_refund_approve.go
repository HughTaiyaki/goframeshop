package order

import (
	"context"

	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func (c *ControllerV1) RefundApprove(ctx context.Context, req *v1.RefundApproveReq) (res *v1.RefundApproveRes, err error) {
	grpcRes, err := c.RefundsClient.ApproveRefund(ctx, &refundv1.ApproveRefundRequest{RefundNo: req.RefundNo, Note: req.Note})
	if err != nil {
		return nil, err
	}
	return &v1.RefundApproveRes{RefundView: toRefundView(grpcRes.Refund)}, nil
}
