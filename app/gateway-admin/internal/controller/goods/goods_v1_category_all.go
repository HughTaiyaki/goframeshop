package goods

import (
	"context"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) CategoryAll(ctx context.Context, req *v1.CategoryAllReq) (res *v1.CategoryAllRes, err error) {
	grpcRes, err := c.CategoriesClient.ListAllCategories(ctx, &categoryv1.ListAllCategoriesRequest{})
	if err != nil {
		return nil, err
	}
	return &v1.CategoryAllRes{Items: toCategoryItems(grpcRes.Items)}, nil
}
