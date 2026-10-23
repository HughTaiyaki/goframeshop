package banner

import (
	"context"

	positionv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) PositionDelete(ctx context.Context, req *v1.PositionDeleteReq) (res *v1.PositionDeleteRes, err error) {
	if _, err := c.PositionsClient.DeletePosition(ctx, &positionv1.DeletePositionRequest{PositionId: req.PositionId}); err != nil {
		return nil, err
	}
	return &v1.PositionDeleteRes{}, nil
}
