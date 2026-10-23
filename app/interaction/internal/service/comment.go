package service

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/model/entity"
)

// CommentInput holds the writable fields of a comment. The author is never
// taken from the request: the caller identity decides who wrote it.
type CommentInput struct {
	ObjectType uint32
	ObjectID   uint64
	ParentID   uint64
	Content    string
}

// IComment is the comment application service. Listing is public, so it takes
// no user id; writing and deleting do.
type IComment interface {
	List(ctx context.Context, objectType uint32, objectID uint64, page, size int) ([]*Comment, int, error)
	Create(ctx context.Context, userID uint64, in *CommentInput) (uint64, error)
	Delete(ctx context.Context, userID, commentID uint64) error
}

// CommentRepository is the persistence seam for comments.
type CommentRepository interface {
	Create(ctx context.Context, c *entity.CommentInfo) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.CommentInfo, error)
	ListByObject(ctx context.Context, objectType uint32, objectID uint64, page, size int) ([]*entity.CommentInfo, error)
	CountByObject(ctx context.Context, objectType uint32, objectID uint64) (int, error)
	SoftDelete(ctx context.Context, userID, id uint64) error
}
