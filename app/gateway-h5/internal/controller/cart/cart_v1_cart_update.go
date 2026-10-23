package cart

import (
	"context"

	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/cart/v1"
)

func (c *ControllerV1) CartUpdate(ctx context.Context, req *v1.CartUpdateReq) (res *v1.CartUpdateRes, err error) {
	grpcRes, err := c.CartClient.UpdateCartItem(ctx, &cartv1.UpdateCartItemRequest{CartItemId: req.CartItemId, Quantity: req.Quantity})
	if err != nil {
		return nil, err
	}
	return &v1.CartUpdateRes{CartItem: toItem(grpcRes.Item)}, nil
}
