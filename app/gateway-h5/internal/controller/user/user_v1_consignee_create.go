package user

import (
	"context"

	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) ConsigneeCreate(ctx context.Context, req *v1.ConsigneeCreateReq) (res *v1.ConsigneeCreateRes, err error) {
	grpcRes, err := c.ConsigneesClient.CreateConsignee(ctx, &consigneev1.CreateConsigneeRequest{
		IsDefault: req.IsDefault,
		Name:      req.Name,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		Town:      req.Town,
		Street:    req.Street,
		Detail:    req.Detail,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ConsigneeCreateRes{ConsigneeId: grpcRes.ConsigneeId}, nil
}
