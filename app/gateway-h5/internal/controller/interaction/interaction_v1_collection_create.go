package interaction

import (
	"context"

	collectionv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/collection/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) CollectionCreate(ctx context.Context, req *v1.CollectionCreateReq) (res *v1.CollectionCreateRes, err error) {
	grpcRes, err := c.CollectionsClient.CreateCollection(ctx, &collectionv1.CreateCollectionRequest{
		ObjectType: req.ObjectType,
		ObjectId:   req.ObjectId,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CollectionCreateRes{CollectionId: grpcRes.CollectionId}, nil
}
