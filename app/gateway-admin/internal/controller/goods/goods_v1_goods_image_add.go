package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsImageAdd(ctx context.Context, req *v1.GoodsImageAddReq) (res *v1.GoodsImageAddRes, err error) {
	grpcRes, err := c.GoodsClient.AddGoodsImage(ctx, &goodsv1.AddGoodsImageRequest{
		GoodsId: req.GoodsId,
		FileId:  req.FileId,
		Sort:    req.Sort,
	})
	if err != nil {
		return nil, err
	}
	return &v1.GoodsImageAddRes{ImageId: grpcRes.ImageId}, nil
}
