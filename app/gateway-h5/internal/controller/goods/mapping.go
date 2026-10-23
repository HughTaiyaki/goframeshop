package goods

import (
	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/goods/v1"
)

func toCategoryItems(categories []*categoryv1.Category) []*v1.CategoryItem {
	items := make([]*v1.CategoryItem, 0, len(categories))
	for _, c := range categories {
		items = append(items, &v1.CategoryItem{
			CategoryId: c.CategoryId,
			ParentId:   c.ParentId,
			Level:      c.Level,
			Name:       c.Name,
			IconFileId: c.IconFileId,
			IconUrl:    c.IconUrl,
			Sort:       c.Sort,
			CreatedAt:  utility.TimeFromTimestamp(c.CreatedAt),
			UpdatedAt:  utility.TimeFromTimestamp(c.UpdatedAt),
		})
	}
	return items
}
