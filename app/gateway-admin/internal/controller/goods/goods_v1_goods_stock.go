package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsStock(ctx context.Context, req *v1.GoodsStockReq) (res *v1.GoodsStockRes, err error) {
	grpcRes, err := c.GoodsClient.AdjustStock(ctx, &goodsv1.AdjustStockRequest{GoodsId: req.GoodsId, Delta: req.Delta})
	if err != nil {
		return nil, err
	}
	return &v1.GoodsStockRes{Available: grpcRes.Available, Reserved: grpcRes.Reserved, Sold: grpcRes.Sold}, nil
}
