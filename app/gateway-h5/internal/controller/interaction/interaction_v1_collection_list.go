package interaction

import (
	"context"

	collectionv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/collection/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) CollectionList(ctx context.Context, req *v1.CollectionListReq) (res *v1.CollectionListRes, err error) {
	grpcRes, err := c.CollectionsClient.ListCollections(ctx, &collectionv1.ListCollectionsRequest{
		ObjectType: req.ObjectType,
		Page:       req.Page,
		Size:       req.Size,
	})
	if err != nil {
		return nil, err
	}

	res = &v1.CollectionListRes{
		Page:  req.Page,
		Size:  req.Size,
		Total: grpcRes.Total,
		Items: make([]*v1.CollectionItem, 0, len(grpcRes.Items)),
	}
	for _, item := range grpcRes.Items {
		res.Items = append(res.Items, &v1.CollectionItem{
			CollectionId: item.CollectionId,
			ObjectType:   item.ObjectType,
			ObjectId:     item.ObjectId,
			CreatedAt:    utility.TimeFromTimestamp(item.CreatedAt),
		})
	}
	return res, nil
}
