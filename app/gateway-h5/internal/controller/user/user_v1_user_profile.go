package user

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) UserProfile(ctx context.Context, req *v1.UserProfileReq) (res *v1.UserProfileRes, err error) {
	grpcRes, err := c.UsersClient.GetProfile(ctx, &userv1.GetProfileRequest{})
	if err != nil {
		return nil, err
	}
	if grpcRes.Profile == nil {
		return nil, gerror.New("user service returned an empty profile")
	}
	return toProfileRes(grpcRes.Profile), nil
}

func toProfileRes(profile *userv1.UserProfile) *v1.UserProfileRes {
	return &v1.UserProfileRes{
		UserId:   profile.UserId,
		Username: profile.Username,
		Avatar:   profile.Avatar,
		Sex:      profile.Sex,
		Sign:     profile.Sign,
		Status:   profile.Status,
		CreateAt: utility.TimeFromTimestamp(profile.CreatedAt),
	}
}
