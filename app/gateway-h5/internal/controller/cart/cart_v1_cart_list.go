package cart

import (
	"context"

	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/cart/v1"
)

func (c *ControllerV1) CartList(ctx context.Context, req *v1.CartListReq) (res *v1.CartListRes, err error) {
	grpcRes, err := c.CartClient.ListCartItems(ctx, &cartv1.ListCartItemsRequest{Page: req.Page, Size: req.Size})
	if err != nil {
		return nil, err
	}
	res = &v1.CartListRes{
		Items: make([]*v1.CartItem, 0, len(grpcRes.Items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: grpcRes.Total,
	}
	for _, item := range grpcRes.Items {
		res.Items = append(res.Items, toItem(item))
	}
	return res, nil
}
