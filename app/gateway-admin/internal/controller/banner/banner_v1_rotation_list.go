package banner

import (
	"context"

	rotationv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/banner/v1"
)

func (c *ControllerV1) RotationList(ctx context.Context, req *v1.RotationListReq) (res *v1.RotationListRes, err error) {
	grpcRes, err := c.RotationsClient.ListRotations(ctx, &rotationv1.ListRotationsRequest{Page: req.Page, Size: req.Size})
	if err != nil {
		return nil, err
	}
	res = &v1.RotationListRes{Items: make([]*v1.RotationItem, 0, len(grpcRes.Items)), Page: req.Page, Size: req.Size, Total: grpcRes.Total}
	for _, x := range grpcRes.Items {
		res.Items = append(res.Items, &v1.RotationItem{
			RotationId: x.RotationId, FileId: x.FileId, PicUrl: x.PicUrl, Link: x.Link, Sort: x.Sort,
			CreatedAt: utility.TimeFromTimestamp(x.CreatedAt), UpdatedAt: utility.TimeFromTimestamp(x.UpdatedAt),
		})
	}
	return res, nil
}
