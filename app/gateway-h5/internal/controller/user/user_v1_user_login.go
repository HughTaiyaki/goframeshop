package user

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) UserLogin(ctx context.Context, req *v1.UserLoginReq) (res *v1.UserLoginRes, err error) {
	grpcRes, err := c.UsersClient.Login(ctx, &userv1.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	if grpcRes.Profile == nil {
		return nil, gerror.New("user service returned an empty profile")
	}
	profile := grpcRes.Profile
	return &v1.UserLoginRes{
		TokenType:   grpcRes.TokenType,
		AccessToken: grpcRes.AccessToken,
		ExpiresIn:   grpcRes.ExpiresInSeconds,
		UserId:      profile.UserId,
		Username:    profile.Username,
		Avatar:      profile.Avatar,
		Sex:         profile.Sex,
		Sign:        profile.Sign,
		Status:      profile.Status,
		CreateAt:    utility.TimeFromTimestamp(profile.CreatedAt),
	}, nil
}
