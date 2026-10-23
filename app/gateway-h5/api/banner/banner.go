// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package banner

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/banner/v1"
)

type IBannerV1 interface {
	RotationList(ctx context.Context, req *v1.RotationListReq) (res *v1.RotationListRes, err error)
	PositionList(ctx context.Context, req *v1.PositionListReq) (res *v1.PositionListRes, err error)
}
