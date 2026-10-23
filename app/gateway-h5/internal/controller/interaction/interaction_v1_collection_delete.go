package interaction

import (
	"context"

	collectionv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/collection/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) CollectionDelete(ctx context.Context, req *v1.CollectionDeleteReq) (res *v1.CollectionDeleteRes, err error) {
	if _, err := c.CollectionsClient.DeleteCollection(ctx, &collectionv1.DeleteCollectionRequest{CollectionId: req.CollectionId}); err != nil {
		return nil, err
	}
	return &v1.CollectionDeleteRes{}, nil
}
