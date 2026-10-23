// Package file implements the file application service.
package file

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/hughtaiyaki/goframeshop/app/file/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/service"
)

type sFile struct {
	repo service.FileRepository
}

func New(repo service.FileRepository) service.IFile {
	return &sFile{repo: repo}
}

func (s *sFile) Create(ctx context.Context, uploaderID uint64, in *service.File) (uint64, error) {
	if uploaderID == 0 ||
		(in.Storage != service.StorageQiniu && in.Storage != service.StorageLocal) ||
		in.ObjectKey == "" || in.URL == "" || in.Size == 0 || in.ContentType == "" ||
		utf8.RuneCountInString(in.Name) > 255 {
		return 0, service.ErrInvalidArgument
	}
	return s.repo.Create(ctx, &entity.FileInfo{
		UploaderId:  uploaderID,
		Name:        strings.TrimSpace(in.Name),
		Storage:     in.Storage,
		ObjectKey:   in.ObjectKey,
		Url:         in.URL,
		Size:        in.Size,
		ContentType: in.ContentType,
	})
}

func (s *sFile) Get(ctx context.Context, fileID uint64) (*service.File, error) {
	record, err := s.repo.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, service.ErrNotFound
	}
	return &service.File{
		ID:          record.Id,
		UploaderID:  record.UploaderId,
		Name:        record.Name,
		Storage:     record.Storage,
		ObjectKey:   record.ObjectKey,
		URL:         record.Url,
		Size:        record.Size,
		ContentType: record.ContentType,
		CreatedAt:   record.CreatedAt,
	}, nil
}
