// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package order

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

type IOrderV1 interface {
	OrderList(ctx context.Context, req *v1.OrderListReq) (res *v1.OrderListRes, err error)
	OrderDetail(ctx context.Context, req *v1.OrderDetailReq) (res *v1.OrderDetailRes, err error)
	RefundList(ctx context.Context, req *v1.RefundListReq) (res *v1.RefundListRes, err error)
}
