package banner

import (
	"context"

	rotationv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) RotationDelete(ctx context.Context, req *v1.RotationDeleteReq) (res *v1.RotationDeleteRes, err error) {
	if _, err := c.RotationsClient.DeleteRotation(ctx, &rotationv1.DeleteRotationRequest{RotationId: req.RotationId}); err != nil {
		return nil, err
	}
	return &v1.RotationDeleteRes{}, nil
}
