package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsImageRemove(ctx context.Context, req *v1.GoodsImageRemoveReq) (res *v1.GoodsImageRemoveRes, err error) {
	if _, err := c.GoodsClient.RemoveGoodsImage(ctx, &goodsv1.RemoveGoodsImageRequest{ImageId: req.ImageId}); err != nil {
		return nil, err
	}
	return &v1.GoodsImageRemoveRes{}, nil
}
