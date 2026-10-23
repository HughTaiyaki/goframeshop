package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsShelf(ctx context.Context, req *v1.GoodsShelfReq) (res *v1.GoodsShelfRes, err error) {
	if _, err := c.GoodsClient.SetGoodsShelf(ctx, &goodsv1.SetGoodsShelfRequest{GoodsId: req.GoodsId, OnShelf: req.OnShelf}); err != nil {
		return nil, err
	}
	return &v1.GoodsShelfRes{}, nil
}
