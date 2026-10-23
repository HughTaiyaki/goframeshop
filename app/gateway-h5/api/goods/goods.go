// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package goods

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/goods/v1"
)

type IGoodsV1 interface {
	GoodsList(ctx context.Context, req *v1.GoodsListReq) (res *v1.GoodsListRes, err error)
	GoodsDetail(ctx context.Context, req *v1.GoodsDetailReq) (res *v1.GoodsDetailRes, err error)
	CategoryList(ctx context.Context, req *v1.CategoryListReq) (res *v1.CategoryListRes, err error)
	CategoryAll(ctx context.Context, req *v1.CategoryAllReq) (res *v1.CategoryAllRes, err error)
}
