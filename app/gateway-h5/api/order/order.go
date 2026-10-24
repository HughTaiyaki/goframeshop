// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package order

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/order/v1"
)

type IOrderV1 interface {
	OrderCreate(ctx context.Context, req *v1.OrderCreateReq) (res *v1.OrderCreateRes, err error)
	OrderCancel(ctx context.Context, req *v1.OrderCancelReq) (res *v1.OrderCancelRes, err error)
	OrderList(ctx context.Context, req *v1.OrderListReq) (res *v1.OrderListRes, err error)
	OrderPay(ctx context.Context, req *v1.OrderPayReq) (res *v1.OrderPayRes, err error)
	PaymentNotify(ctx context.Context, req *v1.PaymentNotifyReq) (res *v1.PaymentNotifyRes, err error)
	OrderDetail(ctx context.Context, req *v1.OrderDetailReq) (res *v1.OrderDetailRes, err error)
	RefundCreate(ctx context.Context, req *v1.RefundCreateReq) (res *v1.RefundCreateRes, err error)
	RefundList(ctx context.Context, req *v1.RefundListReq) (res *v1.RefundListRes, err error)
	RefundDetail(ctx context.Context, req *v1.RefundDetailReq) (res *v1.RefundDetailRes, err error)
	OrderConfirm(ctx context.Context, req *v1.OrderConfirmReq) (res *v1.OrderConfirmRes, err error)
}
