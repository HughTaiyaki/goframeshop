package interaction

import (
	"context"

	praisev1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/praise/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) PraiseList(ctx context.Context, req *v1.PraiseListReq) (res *v1.PraiseListRes, err error) {
	grpcRes, err := c.PraisesClient.ListPraises(ctx, &praisev1.ListPraisesRequest{
		ObjectType: req.ObjectType,
		Page:       req.Page,
		Size:       req.Size,
	})
	if err != nil {
		return nil, err
	}

	res = &v1.PraiseListRes{
		Page:  req.Page,
		Size:  req.Size,
		Total: grpcRes.Total,
		Items: make([]*v1.PraiseItem, 0, len(grpcRes.Items)),
	}
	for _, item := range grpcRes.Items {
		res.Items = append(res.Items, &v1.PraiseItem{
			PraiseId:   item.PraiseId,
			ObjectType: item.ObjectType,
			ObjectId:   item.ObjectId,
			CreatedAt:  utility.TimeFromTimestamp(item.CreatedAt),
		})
	}
	return res, nil
}
