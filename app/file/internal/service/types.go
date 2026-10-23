// Package service defines the application contracts of the file service.
package service

import (
	"context"
	"errors"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/file/internal/model/entity"
)

var (
	ErrNoIdentity      = errors.New("caller identity is missing")
	ErrNotFound        = errors.New("file not found")
	ErrAlreadyExists   = errors.New("file already recorded")
	ErrInvalidArgument = errors.New("invalid argument")
)

// Storage backends a file can live in.
const (
	StorageQiniu = "qiniu"
	StorageLocal = "local"
)

// File is the domain view of an uploaded file.
type File struct {
	ID          uint64
	UploaderID  uint64
	Name        string
	Storage     string
	ObjectKey   string
	URL         string
	Size        uint64
	ContentType string
	CreatedAt   *gtime.Time
}

// IFile is the file application service.
type IFile interface {
	// Create records a file the gateway has already stored. The uploader is
	// the authenticated caller, never a request field.
	Create(ctx context.Context, uploaderID uint64, in *File) (uint64, error)
	Get(ctx context.Context, fileID uint64) (*File, error)
}

// FileRepository is the persistence seam. Create returns ErrAlreadyExists when
// (storage, object_key) is already recorded.
type FileRepository interface {
	Create(ctx context.Context, f *entity.FileInfo) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.FileInfo, error)
}
