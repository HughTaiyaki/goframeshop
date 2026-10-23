package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) GoodsList(ctx context.Context, req *v1.GoodsListReq) (res *v1.GoodsListRes, err error) {
	grpcRes, err := c.GoodsClient.ListGoods(ctx, &goodsv1.ListGoodsRequest{
		CategoryId: req.CategoryId,
		Shelf:      goodsv1.ShelfFilter(req.Shelf),
		HotOnly:    req.IsHot == 1,
		Page:       req.Page,
		Size:       req.Size,
	})
	if err != nil {
		return nil, err
	}
	res = &v1.GoodsListRes{
		Items: make([]*v1.GoodsItem, 0, len(grpcRes.Items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: grpcRes.Total,
	}
	for _, item := range grpcRes.Items {
		res.Items = append(res.Items, &v1.GoodsItem{
			GoodsId:    item.GoodsId,
			Name:       item.Name,
			PriceCents: item.PriceCents,
			Brand:      item.Brand,
			Stock:      item.Stock,
			Sale:       item.Sale,
			Tags:       item.Tags,
			CoverUrl:   item.CoverUrl,
			OnShelf:    item.OnShelf,
			Sort:       item.Sort,
		})
	}
	return res, nil
}
