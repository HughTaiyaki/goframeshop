// Package service defines the application contracts of the admin service.
package service

import (
	"context"
	"errors"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/admin/internal/model/entity"
)

// Sentinel errors. The controller layer maps them to gRPC status codes.
var (
	ErrNoIdentity        = errors.New("caller identity is missing")
	ErrInvalidCredential = errors.New("invalid name or password")
	ErrAdminExists       = errors.New("administrator name already taken")
	ErrPermissionDenied  = errors.New("only a super administrator may do this")
	ErrInvalidArgument   = errors.New("invalid argument")
)

// Password limits. Administrators get a longer minimum than shop users: one
// stolen admin password exposes the whole back office.
const (
	MinPasswordLen = 12
	MaxPasswordLen = 128
	MaxNameLen     = 64
)

// Admin is the domain view of an administrator. It deliberately omits the
// password hash and salt.
type Admin struct {
	ID        uint64
	Name      string
	IsSuper   bool
	CreatedAt *gtime.Time
}

// CreateInput holds a new administrator's fields.
type CreateInput struct {
	Name     string
	Password string
	IsSuper  bool
}

// IAdmin is the administrator application service.
type IAdmin interface {
	Login(ctx context.Context, name, password string) (*Admin, error)
	// Create is called through the API and checks that callerID is a super
	// administrator.
	Create(ctx context.Context, callerID uint64, in *CreateInput) (uint64, error)
	// Bootstrap creates a super administrator without a caller. It is only
	// reachable from the command line, by someone who already holds the
	// database credentials.
	Bootstrap(ctx context.Context, name, password string) (uint64, error)
}

// AdminRepository is the persistence seam. Create returns ErrAdminExists when
// the unique name key rejects a duplicate.
type AdminRepository interface {
	Create(ctx context.Context, a *entity.AdminInfo) (uint64, error)
	FindByName(ctx context.Context, name string) (*entity.AdminInfo, error)
	FindByID(ctx context.Context, id uint64) (*entity.AdminInfo, error)
}
