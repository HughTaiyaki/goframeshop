package user

import (
	"context"

	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) ConsigneeUpdate(ctx context.Context, req *v1.ConsigneeUpdateReq) (res *v1.ConsigneeUpdateRes, err error) {
	_, err = c.ConsigneesClient.UpdateConsignee(ctx, &consigneev1.UpdateConsigneeRequest{
		ConsigneeId: req.ConsigneeId,
		IsDefault:   req.IsDefault,
		Name:        req.Name,
		Phone:       req.Phone,
		Province:    req.Province,
		City:        req.City,
		Town:        req.Town,
		Street:      req.Street,
		Detail:      req.Detail,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ConsigneeUpdateRes{ConsigneeId: req.ConsigneeId}, nil
}
