// Package mark implements the praise and collection application service.
package mark

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
)

type sMark struct {
	repo service.MarkRepository
}

// New returns the service for whichever table repo is bound to.
func New(repo service.MarkRepository) service.IMark {
	return &sMark{repo: repo}
}

func (s *sMark) List(ctx context.Context, userID uint64, objectType uint32, page, size int) ([]*service.Mark, int, error) {
	if !service.ValidObjectType(objectType) {
		return nil, 0, service.ErrInvalidArgument
	}
	total, err := s.repo.CountByUser(ctx, userID, objectType)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.ListByUser(ctx, userID, objectType, page, size)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create relies on the unique key (user_id, object_type, object_id) to reject
// duplicates. Checking first and inserting second would let two concurrent
// requests both pass the check.
func (s *sMark) Create(ctx context.Context, userID uint64, objectType uint32, objectID uint64) (uint64, error) {
	if !service.ValidObjectType(objectType) || objectID == 0 {
		return 0, service.ErrInvalidArgument
	}
	return s.repo.Create(ctx, &service.Mark{UserID: userID, ObjectType: objectType, ObjectID: objectID})
}

// Delete only removes the caller's own row. A missing row and someone else's
// row both return NotFound, so the response never confirms that an id exists.
func (s *sMark) Delete(ctx context.Context, userID, markID uint64) error {
	record, err := s.repo.FindByID(ctx, markID)
	if err != nil {
		return err
	}
	if record == nil || record.UserID != userID {
		return service.ErrNotFound
	}
	return s.repo.Delete(ctx, userID, markID)
}
