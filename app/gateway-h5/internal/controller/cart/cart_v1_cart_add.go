package cart

import (
	"context"

	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/cart/v1"
)

func (c *ControllerV1) CartAdd(ctx context.Context, req *v1.CartAddReq) (res *v1.CartAddRes, err error) {
	grpcRes, err := c.CartClient.AddCartItem(ctx, &cartv1.AddCartItemRequest{GoodsId: req.GoodsId, Quantity: req.Quantity})
	if err != nil {
		return nil, err
	}
	return &v1.CartAddRes{CartItem: toItem(grpcRes.Item)}, nil
}
