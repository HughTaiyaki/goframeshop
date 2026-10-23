package service

import "context"

// IMark is the praise / collection application service.
//
// Every method takes the authenticated userID explicitly: a user only ever sees
// and changes their own marks.
type IMark interface {
	List(ctx context.Context, userID uint64, objectType uint32, page, size int) ([]*Mark, int, error)
	Create(ctx context.Context, userID uint64, objectType uint32, objectID uint64) (uint64, error)
	Delete(ctx context.Context, userID, markID uint64) error
}

// MarkRepository is the persistence seam for one mark table.
//
// It speaks in domain types rather than generated entities because the same
// interface is implemented for two tables with different entity types.
// Create returns ErrAlreadyExists when the unique key rejects a duplicate.
type MarkRepository interface {
	Create(ctx context.Context, m *Mark) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*Mark, error)
	ListByUser(ctx context.Context, userID uint64, objectType uint32, page, size int) ([]*Mark, error)
	CountByUser(ctx context.Context, userID uint64, objectType uint32) (int, error)
	Delete(ctx context.Context, userID, id uint64) error
}
