package interaction

import (
	"context"

	praisev1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/praise/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) PraiseCreate(ctx context.Context, req *v1.PraiseCreateReq) (res *v1.PraiseCreateRes, err error) {
	grpcRes, err := c.PraisesClient.CreatePraise(ctx, &praisev1.CreatePraiseRequest{
		ObjectType: req.ObjectType,
		ObjectId:   req.ObjectId,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PraiseCreateRes{PraiseId: grpcRes.PraiseId}, nil
}
