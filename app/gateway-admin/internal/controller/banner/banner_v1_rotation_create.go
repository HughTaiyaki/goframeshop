package banner

import (
	"context"

	rotationv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) RotationCreate(ctx context.Context, req *v1.RotationCreateReq) (res *v1.RotationCreateRes, err error) {
	grpcRes, err := c.RotationsClient.CreateRotation(ctx, &rotationv1.CreateRotationRequest{FileId: req.FileId, Link: req.Link, Sort: req.Sort})
	if err != nil {
		return nil, err
	}
	return &v1.RotationCreateRes{RotationId: grpcRes.RotationId}, nil
}
