package cart

import (
	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/cart/v1"
)

func toItem(item *cartv1.CartItem) *v1.CartItem {
	if item == nil {
		return nil
	}
	return &v1.CartItem{
		CartItemId: item.CartItemId,
		GoodsId:    item.GoodsId,
		Quantity:   item.Quantity,
		GoodsName:  item.GoodsName,
		PriceCents: item.PriceCents,
		CoverUrl:   item.CoverUrl,
		Available:  item.Available,
		Stock:      item.Stock,
		CreatedAt:  utility.TimeFromTimestamp(item.CreatedAt),
		UpdatedAt:  utility.TimeFromTimestamp(item.UpdatedAt),
	}
}
