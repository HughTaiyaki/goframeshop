package banner

import (
	"context"

	rotationv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) RotationUpdate(ctx context.Context, req *v1.RotationUpdateReq) (res *v1.RotationUpdateRes, err error) {
	_, err = c.RotationsClient.UpdateRotation(ctx, &rotationv1.UpdateRotationRequest{
		RotationId: req.RotationId, FileId: req.FileId, Link: req.Link, Sort: req.Sort,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RotationUpdateRes{}, nil
}
