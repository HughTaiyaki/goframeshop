package goods

import (
	"context"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) CategoryDelete(ctx context.Context, req *v1.CategoryDeleteReq) (res *v1.CategoryDeleteRes, err error) {
	if _, err := c.CategoriesClient.DeleteCategory(ctx, &categoryv1.DeleteCategoryRequest{CategoryId: req.CategoryId}); err != nil {
		return nil, err
	}
	return &v1.CategoryDeleteRes{}, nil
}
