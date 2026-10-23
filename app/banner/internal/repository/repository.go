// Package repository implements the persistence seams with GoFrame DAOs.
//
// Both tables have a deleted_at column, so GoFrame adds "deleted_at IS NULL"
// to every query on them automatically.
package repository

import (
	"context"

	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

type rotationRepository struct{}

func NewRotation() service.RotationRepository { return &rotationRepository{} }

func (r *rotationRepository) Create(ctx context.Context, x *service.Rotation) (uint64, error) {
	id, err := dao.RotationInfo.Ctx(ctx).Data(rotationColumns(x)).InsertAndGetId()
	return uint64(id), err
}

func (r *rotationRepository) Exists(ctx context.Context, id uint64) (bool, error) {
	n, err := dao.RotationInfo.Ctx(ctx).Where(dao.RotationInfo.Columns().Id, id).Count()
	return n > 0, err
}

func (r *rotationRepository) Update(ctx context.Context, x *service.Rotation) error {
	_, err := dao.RotationInfo.Ctx(ctx).Data(rotationColumns(x)).Where(dao.RotationInfo.Columns().Id, x.ID).Update()
	return err
}

func (r *rotationRepository) SoftDelete(ctx context.Context, id uint64) error {
	columns := dao.RotationInfo.Columns()
	_, err := dao.RotationInfo.Ctx(ctx).Data(gdb.Map{columns.DeletedAt: gtime.Now()}).Where(columns.Id, id).Update()
	return err
}

func (r *rotationRepository) Count(ctx context.Context) (int, error) {
	return dao.RotationInfo.Ctx(ctx).Count()
}

// List orders by sort, then id, so equal sort values still page stably.
func (r *rotationRepository) List(ctx context.Context, page, size int) ([]*service.Rotation, error) {
	columns := dao.RotationInfo.Columns()
	all, err := dao.RotationInfo.Ctx(ctx).OrderAsc(columns.Sort).OrderAsc(columns.Id).Page(page, size).All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.RotationInfo
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	out := make([]*service.Rotation, 0, len(rows))
	for _, row := range rows {
		x := &service.Rotation{ID: row.Id, FileID: row.FileId, PicURL: row.PicUrl, Link: row.Link, Sort: uint32(row.Sort)}
		x.CreatedAt, x.UpdatedAt = timeOf(row.CreatedAt), timeOf(row.UpdatedAt)
		out = append(out, x)
	}
	return out, nil
}

// rotationColumns lists the writable columns only.
func rotationColumns(x *service.Rotation) do.RotationInfo {
	return do.RotationInfo{FileId: x.FileID, PicUrl: x.PicURL, Link: x.Link, Sort: x.Sort}
}

type positionRepository struct{}

func NewPosition() service.PositionRepository { return &positionRepository{} }

func (r *positionRepository) Create(ctx context.Context, x *service.Position) (uint64, error) {
	id, err := dao.PositionInfo.Ctx(ctx).Data(positionColumns(x)).InsertAndGetId()
	return uint64(id), err
}

func (r *positionRepository) Exists(ctx context.Context, id uint64) (bool, error) {
	n, err := dao.PositionInfo.Ctx(ctx).Where(dao.PositionInfo.Columns().Id, id).Count()
	return n > 0, err
}

func (r *positionRepository) Update(ctx context.Context, x *service.Position) error {
	_, err := dao.PositionInfo.Ctx(ctx).Data(positionColumns(x)).Where(dao.PositionInfo.Columns().Id, x.ID).Update()
	return err
}

func (r *positionRepository) SoftDelete(ctx context.Context, id uint64) error {
	columns := dao.PositionInfo.Columns()
	_, err := dao.PositionInfo.Ctx(ctx).Data(gdb.Map{columns.DeletedAt: gtime.Now()}).Where(columns.Id, id).Update()
	return err
}

func (r *positionRepository) Count(ctx context.Context, availableOnly bool) (int, error) {
	return r.filtered(ctx, availableOnly).Count()
}

func (r *positionRepository) List(ctx context.Context, page, size int, availableOnly bool) ([]*service.Position, error) {
	columns := dao.PositionInfo.Columns()
	all, err := r.filtered(ctx, availableOnly).OrderAsc(columns.Sort).OrderAsc(columns.Id).Page(page, size).All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.PositionInfo
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	out := make([]*service.Position, 0, len(rows))
	for _, row := range rows {
		x := &service.Position{
			ID: row.Id, FileID: row.FileId, PicURL: row.PicUrl, GoodsID: row.GoodsId,
			GoodsName: row.GoodsName, GoodsOnShelf: row.GoodsOnShelf == 1, Link: row.Link, Sort: uint32(row.Sort),
		}
		x.CreatedAt, x.UpdatedAt = timeOf(row.CreatedAt), timeOf(row.UpdatedAt)
		out = append(out, x)
	}
	return out, nil
}

func (r *positionRepository) filtered(ctx context.Context, availableOnly bool) *gdb.Model {
	model := dao.PositionInfo.Ctx(ctx)
	if availableOnly {
		model = model.Where(dao.PositionInfo.Columns().GoodsOnShelf, 1)
	}
	return model
}

// positionColumns is what an administrator's save writes. The goods was just
// checked to be on sale (catalog.Goods refuses anything else), so the slot
// is available again; goods_version is left alone, so an older goods.changed
// arriving later still cannot overwrite what the administrator saw.
func positionColumns(x *service.Position) do.PositionInfo {
	return do.PositionInfo{FileId: x.FileID, PicUrl: x.PicURL, GoodsId: x.GoodsID, GoodsName: x.GoodsName,
		GoodsOnShelf: 1, Link: x.Link, Sort: x.Sort}
}

// ApplyGoodsChangedTx updates every slot showing the goods, inside tx (the
// banner consumer's inbox transaction), but only where this event is newer
// than the last one applied. It returns how many slots changed.
func ApplyGoodsChangedTx(ctx context.Context, tx gdb.TX, e eventbus.GoodsChanged, version uint64) (int64, error) {
	columns := dao.PositionInfo.Columns()
	onShelf := 0
	if e.OnShelf && !e.Deleted {
		onShelf = 1
	}
	data := gdb.Map{columns.GoodsOnShelf: onShelf, columns.GoodsVersion: version}
	if e.Name != "" {
		data[columns.GoodsName] = e.Name
	}
	result, err := dao.PositionInfo.Ctx(ctx).TX(tx).Data(data).
		Where(columns.GoodsId, e.GoodsID).
		WhereLT(columns.GoodsVersion, version).
		Update()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func timeOf(t *gtime.Time) (out time.Time) {
	if t != nil {
		out = t.Time
	}
	return out
}
