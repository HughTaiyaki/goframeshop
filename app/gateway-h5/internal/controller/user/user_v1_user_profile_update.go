package user

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) UserProfileUpdate(ctx context.Context, req *v1.UserProfileUpdateReq) (res *v1.UserProfileUpdateRes, err error) {
	grpcRes, err := c.UsersClient.UpdateProfile(ctx, &userv1.UpdateProfileRequest{
		Avatar: req.Avatar,
		Sex:    req.Sex,
		Sign:   req.Sign,
	})
	if err != nil {
		return nil, err
	}
	if grpcRes.Profile == nil {
		return nil, gerror.New("user service returned an empty profile")
	}
	return &v1.UserProfileUpdateRes{UserProfileRes: toProfileRes(grpcRes.Profile)}, nil
}
