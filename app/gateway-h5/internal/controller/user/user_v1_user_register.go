package user

import (
	"context"

	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) UserRegister(ctx context.Context, req *v1.UserRegisterReq) (res *v1.UserRegisterRes, err error) {
	grpcRes, err := c.UsersClient.RegisterUser(ctx, &userv1.RegisterRequest{
		Username: req.Username,
		Password: req.Password,
		Avatar:   req.Avatar,
		Sex:      req.Sex,
		Sign:     req.Sign,
	})
	if err != nil {
		return nil, err
	}
	return &v1.UserRegisterRes{UserId: grpcRes.UserId}, nil
}
