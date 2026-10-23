package user

import (
	"context"

	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) ConsigneeDelete(ctx context.Context, req *v1.ConsigneeDeleteReq) (res *v1.ConsigneeDeleteRes, err error) {
	_, err = c.ConsigneesClient.DeleteConsignee(ctx, &consigneev1.DeleteConsigneeRequest{
		ConsigneeId: req.ConsigneeId,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ConsigneeDeleteRes{}, nil
}
