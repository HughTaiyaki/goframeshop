package interaction

import (
	"context"

	praisev1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/praise/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) PraiseDelete(ctx context.Context, req *v1.PraiseDeleteReq) (res *v1.PraiseDeleteRes, err error) {
	if _, err := c.PraisesClient.DeletePraise(ctx, &praisev1.DeletePraiseRequest{PraiseId: req.PraiseId}); err != nil {
		return nil, err
	}
	return &v1.PraiseDeleteRes{}, nil
}
