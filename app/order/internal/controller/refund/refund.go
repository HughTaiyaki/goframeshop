// Package refund is the gRPC transport layer for after-sale requests.
package refund

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

type Controller struct {
	v1.UnimplementedRefundServiceServer
	refunds service.IRefund
}

func Register(s *grpcx.GrpcServer, refunds service.IRefund) {
	v1.RegisterRefundServiceServer(s.Server, &Controller{refunds: refunds})
}

func (c *Controller) CreateRefund(ctx context.Context, req *v1.CreateRefundRequest) (*v1.RefundResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	refund, err := c.refunds.Create(ctx, userID, req.OrderNo, req.GoodsId, req.Reason)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.RefundResponse{Refund: toProto(refund)}, nil
}

func (c *Controller) GetMyRefund(ctx context.Context, req *v1.GetMyRefundRequest) (*v1.RefundResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	refund, err := c.refunds.GetMine(ctx, userID, req.RefundNo)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.RefundResponse{Refund: toProto(refund)}, nil
}

func (c *Controller) ListMyRefunds(ctx context.Context, req *v1.ListMyRefundsRequest) (*v1.ListRefundsResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	refunds, total, err := c.refunds.ListMine(ctx, userID, int(req.Page), int(req.Size))
	return respondList(ctx, refunds, total, req.Page, req.Size, err)
}

func (c *Controller) ListRefunds(ctx context.Context, req *v1.ListRefundsRequest) (*v1.ListRefundsResponse, error) {
	status := ""
	if req.Status != v1.RefundStatus_REFUND_STATUS_UNSPECIFIED {
		status = req.Status.String()
	}
	refunds, total, err := c.refunds.List(ctx, status, int(req.Page), int(req.Size))
	return respondList(ctx, refunds, total, req.Page, req.Size, err)
}

func respondList(ctx context.Context, refunds []*service.Refund, total int, page, size uint32, err error) (*v1.ListRefundsResponse, error) {
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	res := &v1.ListRefundsResponse{Items: make([]*v1.Refund, 0, len(refunds)), Page: page, Size: size, Total: uint64(total)}
	for _, r := range refunds {
		res.Items = append(res.Items, toProto(r))
	}
	return res, nil
}

func toProto(r *service.Refund) *v1.Refund {
	return &v1.Refund{
		RefundNo:    r.No,
		OrderNo:     r.OrderNo,
		UserId:      r.UserID,
		GoodsId:     r.GoodsID,
		Reason:      r.Reason,
		Status:      v1.RefundStatus(v1.RefundStatus_value[r.Status]),
		CreatedAt:   controller.Timestamp(&r.CreatedAt),
		AmountCents: r.AmountCents,
		GoodsName:   r.GoodsName,
		ReviewNote:  r.ReviewNote,
		ReviewedAt:  controller.Timestamp(r.ReviewedAt),
		RefundedAt:  controller.Timestamp(r.RefundedAt),
	}
}

func (c *Controller) ApproveRefund(ctx context.Context, req *v1.ApproveRefundRequest) (*v1.RefundResponse, error) {
	refund, err := c.refunds.Approve(ctx, req.RefundNo, req.Note)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.RefundResponse{Refund: toProto(refund)}, nil
}

func (c *Controller) RejectRefund(ctx context.Context, req *v1.RejectRefundRequest) (*v1.RefundResponse, error) {
	refund, err := c.refunds.Reject(ctx, req.RefundNo, req.Note)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.RefundResponse{Refund: toProto(refund)}, nil
}
