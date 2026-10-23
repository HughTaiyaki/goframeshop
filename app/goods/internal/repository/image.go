package repository

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type imageRepository struct{}

// NewImage returns the MySQL-backed goods image repository.
func NewImage() service.ImageRepository {
	return &imageRepository{}
}

func (r *imageRepository) Create(ctx context.Context, img *entity.GoodsImages) (uint64, error) {
	id, err := dao.GoodsImages.Ctx(ctx).Data(do.GoodsImages{
		GoodsId: img.GoodsId,
		FileId:  img.FileId,
		Url:     img.Url,
		Sort:    img.Sort,
	}).InsertAndGetId()
	if utility.IsDuplicateKey(err) {
		return 0, service.ErrAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *imageRepository) FindByID(ctx context.Context, id uint64) (*entity.GoodsImages, error) {
	one, err := dao.GoodsImages.Ctx(ctx).Where(dao.GoodsImages.Columns().Id, id).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var image entity.GoodsImages
	if err := one.Struct(&image); err != nil {
		return nil, err
	}
	return &image, nil
}

func (r *imageRepository) Delete(ctx context.Context, id uint64) error {
	_, err := dao.GoodsImages.Ctx(ctx).Where(dao.GoodsImages.Columns().Id, id).Delete()
	return err
}

// ListByGoods returns a product's images by sort, then id, so the order is
// stable when two images share a sort value.
func (r *imageRepository) ListByGoods(ctx context.Context, goodsID uint64) ([]*entity.GoodsImages, error) {
	columns := dao.GoodsImages.Columns()
	all, err := dao.GoodsImages.Ctx(ctx).
		Where(columns.GoodsId, goodsID).
		OrderAsc(columns.Sort).
		OrderAsc(columns.Id).
		All()
	if err != nil {
		return nil, err
	}
	images := make([]*entity.GoodsImages, 0, all.Len())
	if err := all.Structs(&images); err != nil {
		return nil, err
	}
	return images, nil
}

// Covers loads the images of a whole page of products in one query and keeps
// the first one per product, instead of one query per product.
func (r *imageRepository) Covers(ctx context.Context, goodsIDs []uint64) (map[uint64]string, error) {
	covers := make(map[uint64]string, len(goodsIDs))
	if len(goodsIDs) == 0 {
		return covers, nil
	}
	columns := dao.GoodsImages.Columns()
	all, err := dao.GoodsImages.Ctx(ctx).
		Fields(columns.GoodsId, columns.Url).
		WhereIn(columns.GoodsId, goodsIDs).
		OrderAsc(columns.GoodsId).
		OrderAsc(columns.Sort).
		OrderAsc(columns.Id).
		All()
	if err != nil {
		return nil, err
	}
	for _, record := range all {
		goodsID := record[columns.GoodsId].Uint64()
		if _, seen := covers[goodsID]; !seen {
			covers[goodsID] = record[columns.Url].String()
		}
	}
	return covers, nil
}
