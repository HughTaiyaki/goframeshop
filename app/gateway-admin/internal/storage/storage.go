// Package storage puts uploaded bytes into an object store.
//
// The gateway only ever writes keys it generated itself, so a storage backend
// never sees a client-chosen path. Metadata (who uploaded what) is recorded by
// the file service, not here.
package storage

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// Storage is one object store.
type Storage interface {
	// Name is recorded with the file, so a URL can be rebuilt or the object
	// deleted later: "qiniu" or "local".
	Name() string
	// Put stores data under key and returns the public URL.
	Put(ctx context.Context, key string, data []byte, contentType string) (url string, err error)
	// Delete removes an object. It is used to clean up when recording the
	// file fails after the bytes were already stored.
	Delete(ctx context.Context, key string) error
}

// DefaultMaxImageBytes is used when upload.maxImageBytes is not set.
const DefaultMaxImageBytes = 5 << 20

// MustNew is New for callers that run after the config was already checked.
func MustNew(ctx context.Context) Storage {
	store, err := New(ctx)
	if err != nil {
		panic(err)
	}
	return store
}

// New builds the backend selected by storage.driver.
//
// Qiniu is used only when chosen explicitly. Local disk is the default so the
// project runs without a cloud account, and so a missing Qiniu setting can
// never silently send files to someone else's bucket.
func New(ctx context.Context) (Storage, error) {
	driver, err := g.Cfg().Get(ctx, "storage.driver", "local")
	if err != nil {
		return nil, gerror.Wrap(err, "read storage.driver")
	}
	switch driver.String() {
	case "local":
		return NewLocalFromConfig(ctx)
	case "qiniu":
		return NewQiniuFromConfig(ctx)
	default:
		return nil, gerror.Newf("storage.driver must be local or qiniu, got %q", driver.String())
	}
}
