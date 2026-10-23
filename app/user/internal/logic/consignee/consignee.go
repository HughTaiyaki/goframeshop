// Package consignee implements the shipping address application service.
package consignee

import (
	"context"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

type sConsignee struct {
	repo service.ConsigneeRepository
}

func New(repo service.ConsigneeRepository) service.IConsignee {
	return &sConsignee{repo: repo}
}

func (s *sConsignee) List(ctx context.Context, userID uint64, page, size int) ([]*service.Consignee, int, error) {
	total, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	records, err := s.repo.ListByUser(ctx, userID, page, size)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*service.Consignee, 0, len(records))
	for _, record := range records {
		items = append(items, toDomain(record))
	}
	return items, total, nil
}

func (s *sConsignee) Create(ctx context.Context, userID uint64, in *service.ConsigneeInput) (uint64, error) {
	if err := validate(in); err != nil {
		return 0, err
	}
	now := gtime.Now()
	record := &entity.ConsigneeInfo{
		UserId:    userID,
		IsDefault: boolToUint(in.IsDefault),
		Name:      in.Name,
		Phone:     in.Phone,
		Province:  in.Province,
		City:      in.City,
		Town:      in.Town,
		Street:    in.Street,
		Detail:    in.Detail,
		CreatedAt: now,
		UpdatedAt: now,
	}

	id, err := s.repo.Create(ctx, record)
	if err != nil {
		return 0, err
	}
	// A newly created default address must demote the previous default. Do it
	// after the insert so the transaction only covers the demotion.
	if in.IsDefault {
		record.Id = id
		if err := s.repo.UpdateAsDefault(ctx, record); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *sConsignee) Update(ctx context.Context, userID, consigneeID uint64, in *service.ConsigneeInput) error {
	if err := validate(in); err != nil {
		return err
	}
	record, err := s.ownedRecord(ctx, userID, consigneeID)
	if err != nil {
		return err
	}

	record.IsDefault = boolToUint(in.IsDefault)
	record.Name = in.Name
	record.Phone = in.Phone
	record.Province = in.Province
	record.City = in.City
	record.Town = in.Town
	record.Street = in.Street
	record.Detail = in.Detail

	if in.IsDefault {
		return s.repo.UpdateAsDefault(ctx, record)
	}
	return s.repo.Update(ctx, record)
}

func (s *sConsignee) Delete(ctx context.Context, userID, consigneeID uint64) error {
	if _, err := s.ownedRecord(ctx, userID, consigneeID); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, userID, consigneeID)
}

func (s *sConsignee) Get(ctx context.Context, userID, consigneeID uint64) (*service.Consignee, error) {
	record, err := s.ownedRecord(ctx, userID, consigneeID)
	if err != nil {
		return nil, err
	}
	return toDomain(record), nil
}

// ownedRecord loads an address and proves it belongs to the caller.
//
// A missing address and someone else's address both return NotFound. Returning
// a distinct "forbidden" would confirm that the id exists, which is a small
// information leak.
func (s *sConsignee) ownedRecord(ctx context.Context, userID, consigneeID uint64) (*entity.ConsigneeInfo, error) {
	record, err := s.repo.FindByID(ctx, consigneeID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.DeletedAt != nil || record.UserId != userID {
		return nil, service.ErrConsigneeNotFound
	}
	return record, nil
}

func validate(in *service.ConsigneeInput) error {
	if in.Name == "" || in.Phone == "" || in.Detail == "" {
		return service.ErrInvalidArgument
	}
	return nil
}

func toDomain(record *entity.ConsigneeInfo) *service.Consignee {
	return &service.Consignee{
		ID:        record.Id,
		UserID:    record.UserId,
		IsDefault: record.IsDefault != 0,
		Name:      record.Name,
		Phone:     record.Phone,
		Province:  record.Province,
		City:      record.City,
		Town:      record.Town,
		Street:    record.Street,
		Detail:    record.Detail,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func boolToUint(flag bool) uint {
	if flag {
		return 1
	}
	return 0
}
