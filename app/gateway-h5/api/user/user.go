// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package user

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

type IUserV1 interface {
	ConsigneeCreate(ctx context.Context, req *v1.ConsigneeCreateReq) (res *v1.ConsigneeCreateRes, err error)
	ConsigneeList(ctx context.Context, req *v1.ConsigneeListReq) (res *v1.ConsigneeListRes, err error)
	ConsigneeUpdate(ctx context.Context, req *v1.ConsigneeUpdateReq) (res *v1.ConsigneeUpdateRes, err error)
	ConsigneeDelete(ctx context.Context, req *v1.ConsigneeDeleteReq) (res *v1.ConsigneeDeleteRes, err error)
	UserRegister(ctx context.Context, req *v1.UserRegisterReq) (res *v1.UserRegisterRes, err error)
	UserLogin(ctx context.Context, req *v1.UserLoginReq) (res *v1.UserLoginRes, err error)
	UserProfile(ctx context.Context, req *v1.UserProfileReq) (res *v1.UserProfileRes, err error)
	UserPasswordUpdate(ctx context.Context, req *v1.UserPasswordUpdateReq) (res *v1.UserPasswordUpdateRes, err error)
	UserProfileUpdate(ctx context.Context, req *v1.UserProfileUpdateReq) (res *v1.UserProfileUpdateRes, err error)
}
