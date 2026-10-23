package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsUpdate(ctx context.Context, req *v1.GoodsUpdateReq) (res *v1.GoodsUpdateRes, err error) {
	_, err = c.GoodsClient.UpdateGoods(ctx, &goodsv1.UpdateGoodsRequest{GoodsId: req.GoodsId, Goods: toGoodsInput(req.GoodsFields)})
	if err != nil {
		return nil, err
	}
	return &v1.GoodsUpdateRes{}, nil
}
