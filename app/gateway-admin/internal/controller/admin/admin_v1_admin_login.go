package admin

import (
	"context"

	adminv1 "github.com/hughtaiyaki/goframeshop/app/admin/api/admin/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/admin/v1"
)

func (c *ControllerV1) AdminLogin(ctx context.Context, req *v1.AdminLoginReq) (res *v1.AdminLoginRes, err error) {
	grpcRes, err := c.AdminsClient.Login(ctx, &adminv1.LoginRequest{Name: req.Name, Password: req.Password})
	if err != nil {
		return nil, err
	}
	return &v1.AdminLoginRes{
		TokenType:   grpcRes.TokenType,
		AccessToken: grpcRes.AccessToken,
		ExpiresIn:   grpcRes.ExpiresInSeconds,
		AdminId:     grpcRes.AdminId,
		Name:        grpcRes.Name,
		IsSuper:     grpcRes.IsSuper,
	}, nil
}
