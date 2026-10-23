package file

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/file/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/service"
)

type fakeFileRepo struct {
	rows map[uint64]*entity.FileInfo
}

func (f *fakeFileRepo) Create(_ context.Context, in *entity.FileInfo) (uint64, error) {
	for _, row := range f.rows {
		if row.Storage == in.Storage && row.ObjectKey == in.ObjectKey {
			return 0, service.ErrAlreadyExists
		}
	}
	id := uint64(len(f.rows) + 1)
	stored := *in
	stored.Id = id
	f.rows[id] = &stored
	return id, nil
}

func (f *fakeFileRepo) FindByID(_ context.Context, id uint64) (*entity.FileInfo, error) {
	return f.rows[id], nil
}

func valid() *service.File {
	return &service.File{
		Name: "a.png", Storage: service.StorageLocal, ObjectKey: "images/k.png",
		URL: "http://x/files/images/k.png", Size: 10, ContentType: "image/png",
	}
}

func TestCreateRecordsTheCallerAsUploader(t *testing.T) {
	ctx := context.Background()
	repo := &fakeFileRepo{rows: map[uint64]*entity.FileInfo{}}
	svc := New(repo)

	id, err := svc.Create(ctx, 7, valid())
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, id)
	if err != nil || got.UploaderID != 7 || got.URL != valid().URL {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := svc.Create(ctx, 7, valid()); !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("duplicate key: err = %v", err)
	}
	if _, err := svc.Get(ctx, 999); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("missing: err = %v", err)
	}
}

func TestCreateRejectsIncompleteRecords(t *testing.T) {
	svc := New(&fakeFileRepo{rows: map[uint64]*entity.FileInfo{}})
	edits := map[string]func(*service.File){
		"unknown storage": func(f *service.File) { f.Storage = "s3" },
		"no key":          func(f *service.File) { f.ObjectKey = "" },
		"no url":          func(f *service.File) { f.URL = "" },
		"zero size":       func(f *service.File) { f.Size = 0 },
		"no type":         func(f *service.File) { f.ContentType = "" },
		"long name":       func(f *service.File) { f.Name = strings.Repeat("名", 256) },
	}
	for name, edit := range edits {
		in := valid()
		edit(in)
		if _, err := svc.Create(context.Background(), 7, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := svc.Create(context.Background(), 0, valid()); !errors.Is(err, service.ErrInvalidArgument) {
		t.Error("uploader 0 accepted")
	}
}
