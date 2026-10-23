// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package banner

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

type IBannerV1 interface {
	RotationList(ctx context.Context, req *v1.RotationListReq) (res *v1.RotationListRes, err error)
	PositionList(ctx context.Context, req *v1.PositionListReq) (res *v1.PositionListRes, err error)
	RotationCreate(ctx context.Context, req *v1.RotationCreateReq) (res *v1.RotationCreateRes, err error)
	RotationUpdate(ctx context.Context, req *v1.RotationUpdateReq) (res *v1.RotationUpdateRes, err error)
	RotationDelete(ctx context.Context, req *v1.RotationDeleteReq) (res *v1.RotationDeleteRes, err error)
	PositionCreate(ctx context.Context, req *v1.PositionCreateReq) (res *v1.PositionCreateRes, err error)
	PositionUpdate(ctx context.Context, req *v1.PositionUpdateReq) (res *v1.PositionUpdateRes, err error)
	PositionDelete(ctx context.Context, req *v1.PositionDeleteReq) (res *v1.PositionDeleteRes, err error)
}
