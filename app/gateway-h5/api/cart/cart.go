// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package cart

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/cart/v1"
)

type ICartV1 interface {
	CartList(ctx context.Context, req *v1.CartListReq) (res *v1.CartListRes, err error)
	CartAdd(ctx context.Context, req *v1.CartAddReq) (res *v1.CartAddRes, err error)
	CartUpdate(ctx context.Context, req *v1.CartUpdateReq) (res *v1.CartUpdateRes, err error)
	CartDelete(ctx context.Context, req *v1.CartDeleteReq) (res *v1.CartDeleteRes, err error)
}
