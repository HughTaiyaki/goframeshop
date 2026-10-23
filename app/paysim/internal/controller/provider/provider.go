// Package provider is the gRPC transport layer of the merchant API.
package provider

import (
	"context"
	"net/url"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/paysim/api/provider/v1"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

type Controller struct {
	v1.UnimplementedPaymentProviderServer
	provider    service.IProvider
	checkoutURL string
}

// Register serves the merchant API. checkoutURL is the base of the payer's
// checkout page, returned to the merchant as pay_url.
func Register(s *grpcx.GrpcServer, provider service.IProvider, checkoutURL string) {
	v1.RegisterPaymentProviderServer(s.Server, &Controller{provider: provider, checkoutURL: checkoutURL})
}

func (c *Controller) CreatePayment(ctx context.Context, req *v1.CreatePaymentRequest) (*v1.PaymentResponse, error) {
	p, err := c.provider.Create(ctx, &service.CreateInput{
		PaymentID: req.PaymentId, OrderNo: req.OrderNo, AmountCents: req.AmountCents,
		Currency: req.Currency, ExpiresAt: req.ExpiresAt.AsTime(),
	})
	return c.respond(ctx, p, err)
}

func (c *Controller) ClosePayment(ctx context.Context, req *v1.ClosePaymentRequest) (*v1.PaymentResponse, error) {
	p, err := c.provider.Close(ctx, req.PaymentId, req.OrderNo)
	return c.respond(ctx, p, err)
}

func (c *Controller) QueryPayment(ctx context.Context, req *v1.QueryPaymentRequest) (*v1.PaymentResponse, error) {
	p, err := c.provider.Query(ctx, req.PaymentId)
	return c.respond(ctx, p, err)
}

func (c *Controller) RefundPayment(ctx context.Context, req *v1.RefundPaymentRequest) (*v1.RefundResponse, error) {
	r, err := c.provider.Refund(ctx, req.RefundId, req.PaymentId, req.AmountCents)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.RefundResponse{RefundId: r.ID, PaymentId: r.PaymentID, AmountCents: r.AmountCents}, nil
}

func (c *Controller) respond(ctx context.Context, p *service.Payment, err error) (*v1.PaymentResponse, error) {
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	out := &v1.Payment{
		PaymentId:    p.ID,
		OrderNo:      p.OrderNo,
		AmountCents:  p.AmountCents,
		Currency:     p.Currency,
		State:        v1.PaymentState(v1.PaymentState_value[p.State]),
		ProviderTxId: p.TxID,
		ExpiresAt:    controller.Timestamp(p.ExpiresAt),
		PaidAt:       controller.Timestamp(p.PaidAt),
	}
	if p.AmountCents > 0 {
		out.PayUrl = c.checkoutURL + "/checkout/" + url.PathEscape(p.ID)
	}
	return &v1.PaymentResponse{Payment: out}, nil
}
