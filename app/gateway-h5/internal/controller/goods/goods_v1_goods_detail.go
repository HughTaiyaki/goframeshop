package goods

import (
	"context"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/goods/v1"
)

func (c *ControllerV1) GoodsDetail(ctx context.Context, req *v1.GoodsDetailReq) (res *v1.GoodsDetailRes, err error) {
	grpcRes, err := c.GoodsClient.GetGoods(ctx, &goodsv1.GetGoodsRequest{GoodsId: req.GoodsId})
	if err != nil {
		return nil, err
	}
	g := grpcRes.GetGoods()
	res = &v1.GoodsDetailRes{
		GoodsId:          g.GetGoodsId(),
		Name:             g.GetName(),
		PriceCents:       g.GetPriceCents(),
		Level1CategoryId: g.GetLevel1CategoryId(),
		Level2CategoryId: g.GetLevel2CategoryId(),
		Level3CategoryId: g.GetLevel3CategoryId(),
		Brand:            g.GetBrand(),
		Stock:            g.GetStock(),
		Sale:             g.GetSale(),
		Tags:             g.GetTags(),
		DetailInfo:       g.GetDetailInfo(),
		Images:           make([]*v1.GoodsImageItem, 0, len(g.GetImages())),
		CreatedAt:        utility.TimeFromTimestamp(g.GetCreatedAt()),
		UpdatedAt:        utility.TimeFromTimestamp(g.GetUpdatedAt()),
	}
	for _, image := range g.GetImages() {
		res.Images = append(res.Images, &v1.GoodsImageItem{
			ImageId: image.ImageId, FileId: image.FileId, Url: image.Url, Sort: image.Sort,
		})
	}
	return res, nil
}
