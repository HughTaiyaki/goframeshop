// Package admin implements the administrator application service.
package admin

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/hughtaiyaki/goframeshop/app/admin/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type sAdmin struct {
	repo service.AdminRepository
}

func New(repo service.AdminRepository) service.IAdmin {
	return &sAdmin{repo: repo}
}

// Login returns the same error for an unknown name and a wrong password, so
// the endpoint cannot be used to discover which administrator names exist.
func (s *sAdmin) Login(ctx context.Context, name, password string) (*service.Admin, error) {
	record, err := s.repo.FindByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	if record == nil || record.DeletedAt != nil ||
		!utility.VerifyPassword(password, record.PasswordHash, record.PasswordSalt) {
		return nil, service.ErrInvalidCredential
	}
	return toDomain(record), nil
}

func (s *sAdmin) Create(ctx context.Context, callerID uint64, in *service.CreateInput) (uint64, error) {
	// The permission check reads the caller from the database on every call,
	// not from a token claim, so demoting or deleting an administrator takes
	// effect immediately instead of when their token expires.
	caller, err := s.repo.FindByID(ctx, callerID)
	if err != nil {
		return 0, err
	}
	if caller == nil || caller.DeletedAt != nil || caller.IsSuper == 0 {
		return 0, service.ErrPermissionDenied
	}
	return s.create(ctx, in)
}

func (s *sAdmin) Bootstrap(ctx context.Context, name, password string) (uint64, error) {
	return s.create(ctx, &service.CreateInput{Name: name, Password: password, IsSuper: true})
}

func (s *sAdmin) create(ctx context.Context, in *service.CreateInput) (uint64, error) {
	name := strings.TrimSpace(in.Name)
	passwordLen := utf8.RuneCountInString(in.Password)
	if name == "" || utf8.RuneCountInString(name) > service.MaxNameLen ||
		passwordLen < service.MinPasswordLen || passwordLen > service.MaxPasswordLen {
		return 0, service.ErrInvalidArgument
	}
	hash, salt, err := utility.HashPassword(in.Password)
	if err != nil {
		return 0, err
	}
	var isSuper uint
	if in.IsSuper {
		isSuper = 1
	}
	return s.repo.Create(ctx, &entity.AdminInfo{
		Name:         name,
		PasswordHash: hash,
		PasswordSalt: salt,
		IsSuper:      isSuper,
	})
}

func toDomain(record *entity.AdminInfo) *service.Admin {
	return &service.Admin{
		ID:        record.Id,
		Name:      record.Name,
		IsSuper:   record.IsSuper != 0,
		CreatedAt: record.CreatedAt,
	}
}
