package order

import (
	"context"

	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func (c *ControllerV1) RefundList(ctx context.Context, req *v1.RefundListReq) (res *v1.RefundListRes, err error) {
	grpcRes, err := c.RefundsClient.ListRefunds(ctx, &refundv1.ListRefundsRequest{
		Status: refundv1.RefundStatus(refundv1.RefundStatus_value[req.Status]),
		Page:   req.Page,
		Size:   req.Size,
	})
	if err != nil {
		return nil, err
	}
	return toRefundList(grpcRes), nil
}
