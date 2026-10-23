// Package file is the gRPC transport layer for file records.
package file

import (
	"context"
	"errors"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedFileServiceServer
	files service.IFile
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterFileServiceServer(s.Server, &Controller{files: logic.File()})
}

func (c *Controller) CreateFile(ctx context.Context, req *v1.CreateFileRequest) (*v1.CreateFileResponse, error) {
	uploaderID, ok := utility.UserIDFromContext(ctx)
	if !ok {
		return nil, statusError(service.ErrNoIdentity)
	}
	id, err := c.files.Create(ctx, uploaderID, &service.File{
		Name:        req.Name,
		Storage:     req.Storage,
		ObjectKey:   req.ObjectKey,
		URL:         req.Url,
		Size:        req.Size,
		ContentType: req.ContentType,
	})
	if err != nil {
		return nil, statusError(err)
	}
	return &v1.CreateFileResponse{FileId: id}, nil
}

func (c *Controller) GetFile(ctx context.Context, req *v1.GetFileRequest) (*v1.GetFileResponse, error) {
	if _, ok := utility.UserIDFromContext(ctx); !ok {
		return nil, statusError(service.ErrNoIdentity)
	}
	f, err := c.files.Get(ctx, req.FileId)
	if err != nil {
		return nil, statusError(err)
	}
	return &v1.GetFileResponse{File: &v1.File{
		FileId:      f.ID,
		UploaderId:  f.UploaderID,
		Name:        f.Name,
		Url:         f.URL,
		Size:        f.Size,
		ContentType: f.ContentType,
		CreatedAt:   utility.SafeConvertTime(f.CreatedAt),
	}}, nil
}

// statusError translates an application error into a gRPC status. Unknown
// errors collapse into a fixed Internal message.
func statusError(err error) error {
	switch {
	case errors.Is(err, service.ErrNoIdentity):
		return status.Error(codes.Unauthenticated, "caller identity is missing")
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, service.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "resource already exists")
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
