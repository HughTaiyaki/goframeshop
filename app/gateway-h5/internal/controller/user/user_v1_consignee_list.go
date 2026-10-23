package user

import (
	"context"

	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/user/v1"
)

func (c *ControllerV1) ConsigneeList(ctx context.Context, req *v1.ConsigneeListReq) (res *v1.ConsigneeListRes, err error) {
	grpcRes, err := c.ConsigneesClient.ListConsignees(ctx, &consigneev1.ListConsigneesRequest{
		Page: req.Page,
		Size: req.Size,
	})
	if err != nil {
		return nil, err
	}

	res = &v1.ConsigneeListRes{
		Page:  req.Page,
		Size:  req.Size,
		Total: grpcRes.Total,
		Items: make([]*v1.ConsigneeItem, 0, len(grpcRes.Items)),
	}
	for _, item := range grpcRes.Items {
		res.Items = append(res.Items, &v1.ConsigneeItem{
			ConsigneeId: item.ConsigneeId,
			IsDefault:   item.IsDefault,
			Name:        item.Name,
			Phone:       item.Phone,
			Province:    item.Province,
			City:        item.City,
			Town:        item.Town,
			Street:      item.Street,
			Detail:      item.Detail,
			CreatedAt:   utility.TimeFromTimestamp(item.CreatedAt),
			UpdatedAt:   utility.TimeFromTimestamp(item.UpdatedAt),
		})
	}
	return res, nil
}
