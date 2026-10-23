package banner

import (
	"context"

	positionv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) PositionUpdate(ctx context.Context, req *v1.PositionUpdateReq) (res *v1.PositionUpdateRes, err error) {
	_, err = c.PositionsClient.UpdatePosition(ctx, &positionv1.UpdatePositionRequest{
		PositionId: req.PositionId, FileId: req.FileId, GoodsId: req.GoodsId, Link: req.Link, Sort: req.Sort,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PositionUpdateRes{}, nil
}
