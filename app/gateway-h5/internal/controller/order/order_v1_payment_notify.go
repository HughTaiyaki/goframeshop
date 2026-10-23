package order

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	paymentv1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

// PaymentNotify passes the exact bytes and the signature header through: the
// signature covers the raw body, so it must not be re-encoded here. A
// response with code 0 is the acknowledgement; anything else makes the
// simulator try again later.
func (c *ControllerV1) PaymentNotify(ctx context.Context, req *v1.PaymentNotifyReq) (res *v1.PaymentNotifyRes, err error) {
	r := g.RequestFromCtx(ctx)
	_, err = c.PaymentsClient.NotifyPayment(ctx, &paymentv1.NotifyPaymentRequest{
		Body:      r.GetBody(),
		Signature: r.Header.Get(utility.PaySignatureHeader),
	})
	if err != nil {
		return nil, err
	}
	return &v1.PaymentNotifyRes{}, nil
}
