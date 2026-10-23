package user

import (
	"context"

	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) UserPasswordUpdate(ctx context.Context, req *v1.UserPasswordUpdateReq) (res *v1.UserPasswordUpdateRes, err error) {
	_, err = c.UsersClient.ChangePassword(ctx, &userv1.ChangePasswordRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		return nil, err
	}
	return &v1.UserPasswordUpdateRes{}, nil
}
