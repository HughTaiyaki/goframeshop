package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsCreate(ctx context.Context, req *v1.GoodsCreateReq) (res *v1.GoodsCreateRes, err error) {
	grpcRes, err := c.GoodsClient.CreateGoods(ctx, &goodsv1.CreateGoodsRequest{
		Goods:        toGoodsInput(req.GoodsFields),
		InitialStock: req.InitialStock,
	})
	if err != nil {
		return nil, err
	}
	return &v1.GoodsCreateRes{GoodsId: grpcRes.GoodsId}, nil
}
