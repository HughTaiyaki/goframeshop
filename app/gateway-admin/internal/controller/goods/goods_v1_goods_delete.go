package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsDelete(ctx context.Context, req *v1.GoodsDeleteReq) (res *v1.GoodsDeleteRes, err error) {
	if _, err := c.GoodsClient.DeleteGoods(ctx, &goodsv1.DeleteGoodsRequest{GoodsId: req.GoodsId}); err != nil {
		return nil, err
	}
	return &v1.GoodsDeleteRes{}, nil
}
