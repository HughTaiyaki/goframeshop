package cart

import (
	"context"

	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/cart/v1"
)

func (c *ControllerV1) CartDelete(ctx context.Context, req *v1.CartDeleteReq) (res *v1.CartDeleteRes, err error) {
	grpcRes, err := c.CartClient.DeleteCartItems(ctx, &cartv1.DeleteCartItemsRequest{CartItemIds: req.CartItemIds})
	if err != nil {
		return nil, err
	}
	return &v1.CartDeleteRes{Deleted: grpcRes.Deleted}, nil
}
