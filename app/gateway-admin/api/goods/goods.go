// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package goods

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

type IGoodsV1 interface {
	GoodsList(ctx context.Context, req *v1.GoodsListReq) (res *v1.GoodsListRes, err error)
	GoodsDetail(ctx context.Context, req *v1.GoodsDetailReq) (res *v1.GoodsDetailRes, err error)
	CategoryList(ctx context.Context, req *v1.CategoryListReq) (res *v1.CategoryListRes, err error)
	CategoryAll(ctx context.Context, req *v1.CategoryAllReq) (res *v1.CategoryAllRes, err error)
	GoodsCreate(ctx context.Context, req *v1.GoodsCreateReq) (res *v1.GoodsCreateRes, err error)
	GoodsUpdate(ctx context.Context, req *v1.GoodsUpdateReq) (res *v1.GoodsUpdateRes, err error)
	GoodsDelete(ctx context.Context, req *v1.GoodsDeleteReq) (res *v1.GoodsDeleteRes, err error)
	GoodsImageAdd(ctx context.Context, req *v1.GoodsImageAddReq) (res *v1.GoodsImageAddRes, err error)
	GoodsImageRemove(ctx context.Context, req *v1.GoodsImageRemoveReq) (res *v1.GoodsImageRemoveRes, err error)
	CategoryCreate(ctx context.Context, req *v1.CategoryCreateReq) (res *v1.CategoryCreateRes, err error)
	CategoryUpdate(ctx context.Context, req *v1.CategoryUpdateReq) (res *v1.CategoryUpdateRes, err error)
	CategoryDelete(ctx context.Context, req *v1.CategoryDeleteReq) (res *v1.CategoryDeleteRes, err error)
	GoodsShelf(ctx context.Context, req *v1.GoodsShelfReq) (res *v1.GoodsShelfRes, err error)
	GoodsStock(ctx context.Context, req *v1.GoodsStockReq) (res *v1.GoodsStockRes, err error)
}
