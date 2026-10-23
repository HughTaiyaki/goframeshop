package file

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	filev1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	v1 "github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/file/v1"
)

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)

type fakeStore struct {
	objects map[string][]byte
	putErr  error
}

func (f *fakeStore) Name() string { return "local" }
func (f *fakeStore) Put(_ context.Context, key string, data []byte, _ string) (string, error) {
	if f.putErr != nil {
		return "", f.putErr
	}
	f.objects[key] = data
	return "http://files/" + key, nil
}
func (f *fakeStore) Delete(_ context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

type fakeFiles struct {
	filev1.FileServiceClient
	got *filev1.CreateFileRequest
	err error
}

func (f *fakeFiles) CreateFile(_ context.Context, in *filev1.CreateFileRequest, _ ...grpc.CallOption) (*filev1.CreateFileResponse, error) {
	f.got = in
	if f.err != nil {
		return nil, f.err
	}
	return &filev1.CreateFileResponse{FileId: 9}, nil
}

// uploadFile builds a real multipart upload, the way a browser sends it.
func uploadFile(t *testing.T, name string, data []byte) *ghttp.UploadFile {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = w.Close()
	req, _ := http.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return &ghttp.UploadFile{FileHeader: req.MultipartForm.File["file"][0]}
}

func TestUploadImageStoresThenRecords(t *testing.T) {
	store := &fakeStore{objects: map[string][]byte{}}
	files := &fakeFiles{}
	c := &ControllerV1{FilesClient: files, Store: store, MaxImageBytes: 1024}

	res, err := c.UploadImage(context.Background(), &v1.UploadImageReq{File: uploadFile(t, "../商品图.png", pngBytes)})
	if err != nil {
		t.Fatal(err)
	}
	if res.FileId != 9 || len(store.objects) != 1 {
		t.Fatalf("res=%+v objects=%d", res, len(store.objects))
	}
	if files.got.Name != "商品图.png" || files.got.Storage != "local" ||
		files.got.ContentType != "image/png" || files.got.Size != uint64(len(pngBytes)) || files.got.Url != res.Url {
		t.Fatalf("recorded %+v", files.got)
	}
}

func TestUploadImageRemovesTheObjectWhenRecordingFails(t *testing.T) {
	store := &fakeStore{objects: map[string][]byte{}}
	files := &fakeFiles{err: status.Error(codes.Unavailable, "file service down")}
	c := &ControllerV1{FilesClient: files, Store: store, MaxImageBytes: 1024}

	if _, err := c.UploadImage(context.Background(), &v1.UploadImageReq{File: uploadFile(t, "a.png", pngBytes)}); err == nil {
		t.Fatal("expected an error")
	}
	if len(store.objects) != 0 {
		t.Fatal("object left behind with no file record pointing at it")
	}
}

func TestUploadImageRejectsBeforeStoring(t *testing.T) {
	store := &fakeStore{objects: map[string][]byte{}}
	files := &fakeFiles{}
	c := &ControllerV1{FilesClient: files, Store: store, MaxImageBytes: 16}

	for name, data := range map[string][]byte{
		"too large": pngBytes,
		"html":      []byte("<html>hi</html>"),
	} {
		_, err := c.UploadImage(context.Background(), &v1.UploadImageReq{File: uploadFile(t, "a.png", data)})
		if gerror.Code(err).Code() != int(codes.InvalidArgument) {
			t.Errorf("%s: code = %d, want %d", name, gerror.Code(err).Code(), codes.InvalidArgument)
		}
	}
	if len(store.objects) != 0 || files.got != nil {
		t.Fatal("a rejected file reached storage or the file service")
	}
}

func TestUploadImageHidesStorageErrors(t *testing.T) {
	store := &fakeStore{objects: map[string][]byte{}, putErr: errors.New("qiniu: bad token for bucket secret-bucket")}
	c := &ControllerV1{FilesClient: &fakeFiles{}, Store: store, MaxImageBytes: 1024}

	_, err := c.UploadImage(context.Background(), &v1.UploadImageReq{File: uploadFile(t, "a.png", pngBytes)})
	if err == nil || err.Error() != "image upload failed" {
		t.Fatalf("err = %v, want the generic message", err)
	}
}
