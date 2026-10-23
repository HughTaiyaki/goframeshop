package banner

import (
	"context"

	positionv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) PositionCreate(ctx context.Context, req *v1.PositionCreateReq) (res *v1.PositionCreateRes, err error) {
	grpcRes, err := c.PositionsClient.CreatePosition(ctx, &positionv1.CreatePositionRequest{
		FileId: req.FileId, GoodsId: req.GoodsId, Link: req.Link, Sort: req.Sort,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PositionCreateRes{PositionId: grpcRes.PositionId}, nil
}
