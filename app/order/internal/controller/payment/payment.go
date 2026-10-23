// Package payment is the gRPC transport layer for paying orders.
package payment

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/controller"
	ordercontroller "github.com/hughtaiyaki/goframeshop/app/order/internal/controller/order"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

type Controller struct {
	v1.UnimplementedPaymentServiceServer
	orders service.IOrder
}

func Register(s *grpcx.GrpcServer, orders service.IOrder) {
	v1.RegisterPaymentServiceServer(s.Server, &Controller{orders: orders})
}

func (c *Controller) PayOrder(ctx context.Context, req *v1.PayOrderRequest) (*v1.PayOrderResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	view, err := c.orders.Pay(ctx, userID, req.OrderNo)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.PayOrderResponse{
		PaymentId:   view.PaymentID,
		State:       view.State,
		AmountCents: view.AmountCents,
		PayUrl:      view.PayURL,
		ExpiresAt:   controller.Timestamp(view.ExpiresAt),
		Order:       ordercontroller.ToProto(view.Order),
	}, nil
}

func (c *Controller) NotifyPayment(ctx context.Context, req *v1.NotifyPaymentRequest) (*v1.NotifyPaymentResponse, error) {
	if err := c.orders.Notify(ctx, req.Body, req.Signature); err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.NotifyPaymentResponse{}, nil
}
