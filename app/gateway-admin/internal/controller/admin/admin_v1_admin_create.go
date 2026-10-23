package admin

import (
	"context"

	adminv1 "github.com/hughtaiyaki/goframeshop/app/admin/api/admin/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/admin/v1"
)

func (c *ControllerV1) AdminCreate(ctx context.Context, req *v1.AdminCreateReq) (res *v1.AdminCreateRes, err error) {
	grpcRes, err := c.AdminsClient.CreateAdmin(ctx, &adminv1.CreateAdminRequest{
		Name:     req.Name,
		Password: req.Password,
		IsSuper:  req.IsSuper,
	})
	if err != nil {
		return nil, err
	}
	return &v1.AdminCreateRes{AdminId: grpcRes.AdminId}, nil
}
