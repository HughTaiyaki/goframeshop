package file

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"google.golang.org/grpc/codes"

	filev1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/upload"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/file/v1"
)

// codeInvalidArgument reuses the gRPC code number so a bad upload looks the
// same to the client as any other invalid argument.
var codeInvalidArgument = gcode.New(int(codes.InvalidArgument), "InvalidArgument", nil)

// UploadImage validates the image, stores it, then asks the file service to
// record it. The uploader id is taken by the file service from the forwarded
// administrator token, never from this request.
func (c *ControllerV1) UploadImage(ctx context.Context, req *v1.UploadImageReq) (res *v1.UploadImageRes, err error) {
	if req.File == nil {
		return nil, gerror.NewCode(codeInvalidArgument, "file is required")
	}
	f, err := req.File.Open()
	if err != nil {
		return nil, gerror.NewCode(codeInvalidArgument, "cannot read the uploaded file")
	}
	defer f.Close()

	img, err := upload.ReadImage(f, req.File.Filename, c.MaxImageBytes, time.Now())
	switch {
	case errors.Is(err, upload.ErrTooLarge):
		return nil, gerror.NewCode(codeInvalidArgument, fmt.Sprintf("image must be at most %d bytes", c.MaxImageBytes))
	case errors.Is(err, upload.ErrUnsupportedType), errors.Is(err, upload.ErrEmpty):
		return nil, gerror.NewCode(codeInvalidArgument, err.Error())
	case err != nil:
		return nil, gerror.NewCode(gcode.CodeInternalError, "cannot read the uploaded file")
	}

	url, err := c.Store.Put(ctx, img.Key, img.Data, img.ContentType)
	if err != nil {
		// The cause stays in the log; the client only learns that it failed.
		g.Log().Errorf(ctx, "image upload to %s failed: %v", c.Store.Name(), err)
		return nil, gerror.NewCode(gcode.CodeInternalError, "image upload failed")
	}

	created, err := c.FilesClient.CreateFile(ctx, &filev1.CreateFileRequest{
		Name:        img.DisplayName,
		Storage:     c.Store.Name(),
		ObjectKey:   img.Key,
		Url:         url,
		Size:        uint64(len(img.Data)),
		ContentType: img.ContentType,
	})
	if err != nil {
		// The bytes are stored but nothing points at them. Remove them so the
		// bucket does not fill with orphans; if that fails too, log the key so
		// it can be cleaned up by hand.
		if delErr := c.Store.Delete(ctx, img.Key); delErr != nil {
			g.Log().Errorf(ctx, "orphaned object %s in %s: %v", img.Key, c.Store.Name(), delErr)
		}
		return nil, err
	}
	return &v1.UploadImageRes{FileId: created.FileId, Url: url}, nil
}
