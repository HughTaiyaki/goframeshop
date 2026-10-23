// Package comment implements the comment application service.
package comment

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
)

// maxContentRunes matches content VARCHAR(255): MySQL counts characters, not
// bytes, so the limit is checked in runes.
const maxContentRunes = 255

type sComment struct {
	repo service.CommentRepository
}

func New(repo service.CommentRepository) service.IComment {
	return &sComment{repo: repo}
}

func (s *sComment) List(ctx context.Context, objectType uint32, objectID uint64, page, size int) ([]*service.Comment, int, error) {
	if !service.ValidObjectType(objectType) || objectID == 0 {
		return nil, 0, service.ErrInvalidArgument
	}
	total, err := s.repo.CountByObject(ctx, objectType, objectID)
	if err != nil {
		return nil, 0, err
	}
	records, err := s.repo.ListByObject(ctx, objectType, objectID, page, size)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*service.Comment, 0, len(records))
	for _, record := range records {
		items = append(items, toDomain(record))
	}
	return items, total, nil
}

func (s *sComment) Create(ctx context.Context, userID uint64, in *service.CommentInput) (uint64, error) {
	content := strings.TrimSpace(in.Content)
	if !service.ValidObjectType(in.ObjectType) || in.ObjectID == 0 ||
		content == "" || utf8.RuneCountInString(content) > maxContentRunes {
		return 0, service.ErrInvalidArgument
	}
	// A reply must hang off a live comment on the same object. Otherwise a
	// client could attach a reply to a comment on a different product, or to
	// a deleted one, and the tree would no longer make sense.
	if in.ParentID != 0 {
		parent, err := s.repo.FindByID(ctx, in.ParentID)
		if err != nil {
			return 0, err
		}
		if parent == nil || parent.DeletedAt != nil ||
			uint32(parent.ObjectType) != in.ObjectType || parent.ObjectId != in.ObjectID {
			return 0, service.ErrInvalidArgument
		}
	}
	return s.repo.Create(ctx, &entity.CommentInfo{
		ParentId:   in.ParentID,
		UserId:     userID,
		ObjectType: uint(in.ObjectType),
		ObjectId:   in.ObjectID,
		Content:    content,
	})
}

// Delete lets only the author remove a comment. Replies stay: they are the
// other users' content, and the client can render them under a placeholder.
func (s *sComment) Delete(ctx context.Context, userID, commentID uint64) error {
	record, err := s.repo.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	if record == nil || record.DeletedAt != nil || record.UserId != userID {
		return service.ErrNotFound
	}
	return s.repo.SoftDelete(ctx, userID, commentID)
}

func toDomain(record *entity.CommentInfo) *service.Comment {
	return &service.Comment{
		ID:         record.Id,
		ParentID:   record.ParentId,
		UserID:     record.UserId,
		ObjectType: uint32(record.ObjectType),
		ObjectID:   record.ObjectId,
		Content:    record.Content,
		CreatedAt:  record.CreatedAt,
	}
}
