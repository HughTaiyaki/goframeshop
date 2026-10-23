package banner

import (
	"context"

	positionv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) PositionList(ctx context.Context, req *v1.PositionListReq) (res *v1.PositionListRes, err error) {
	grpcRes, err := c.PositionsClient.ListPositions(ctx, &positionv1.ListPositionsRequest{Page: req.Page, Size: req.Size})
	if err != nil {
		return nil, err
	}
	res = &v1.PositionListRes{Items: make([]*v1.PositionItem, 0, len(grpcRes.Items)), Page: req.Page, Size: req.Size, Total: grpcRes.Total}
	for _, x := range grpcRes.Items {
		res.Items = append(res.Items, &v1.PositionItem{
			PositionId: x.PositionId, FileId: x.FileId, PicUrl: x.PicUrl, GoodsId: x.GoodsId, GoodsName: x.GoodsName,
			GoodsOnShelf: x.GoodsOnShelf, Link: x.Link, Sort: x.Sort,
			CreatedAt: utility.TimeFromTimestamp(x.CreatedAt), UpdatedAt: utility.TimeFromTimestamp(x.UpdatedAt),
		})
	}
	return res, nil
}
