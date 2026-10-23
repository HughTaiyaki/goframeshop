// Package user implements the user application service.
package user

import (
	"context"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// StatusActive mirrors the status column comment in the schema.
const statusActive = 1

type sUser struct {
	repo service.UserRepository
}

// New builds a user service around the given repository. The repository is
// injected rather than imported so tests can supply an in-memory fake.
func New(repo service.UserRepository) service.IUser {
	return &sUser{repo: repo}
}

func (s *sUser) Register(ctx context.Context, in *service.RegisterInput) (*service.User, error) {
	// The transport layer already checked length; check again because the
	// application service must stay correct even if it is called from somewhere
	// else (a job, a test, another service).
	if in.Username == "" || len(in.Password) < 8 {
		return nil, service.ErrInvalidArgument
	}
	profile, err := checkProfile(&service.ProfileInput{Avatar: in.Avatar, Sex: in.Sex, Sign: in.Sign})
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByName(ctx, in.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, service.ErrUserExists
	}

	hash, salt, err := utility.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	now := gtime.Now()
	id, err := s.repo.Create(ctx, &entity.UserInfo{
		Name:         in.Username,
		Avatar:       profile.Avatar,
		PasswordHash: hash,
		PasswordSalt: salt,
		Sex:          profile.Sex,
		Status:       statusActive,
		Sign:         profile.Sign,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return nil, err
	}

	return &service.User{
		ID:        id,
		Name:      in.Username,
		Avatar:    profile.Avatar,
		Sex:       profile.Sex,
		Status:    statusActive,
		Sign:      profile.Sign,
		CreatedAt: now,
	}, nil
}

func (s *sUser) Login(ctx context.Context, username, password string) (*service.User, error) {
	record, err := s.repo.FindByName(ctx, username)
	if err != nil {
		return nil, err
	}
	// One error for both "no such user" and "wrong password": telling them apart
	// would let anyone enumerate which usernames exist.
	if record == nil || !utility.VerifyPassword(password, record.PasswordHash, record.PasswordSalt) {
		return nil, service.ErrInvalidCredential
	}
	if record.Status != statusActive {
		return nil, service.ErrUserBlocked
	}
	return toDomain(record), nil
}

func (s *sUser) GetProfile(ctx context.Context, userID uint64) (*service.User, error) {
	record, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, service.ErrUserNotFound
	}
	return toDomain(record), nil
}

func (s *sUser) ChangePassword(ctx context.Context, userID uint64, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return service.ErrInvalidArgument
	}
	record, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if record == nil {
		return service.ErrUserNotFound
	}
	// Changing a password is as sensitive as logging in: require the old one so
	// a stolen session alone cannot lock the real owner out.
	if !utility.VerifyPassword(currentPassword, record.PasswordHash, record.PasswordSalt) {
		return service.ErrInvalidCredential
	}
	hash, salt, err := utility.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdateCredential(ctx, userID, hash, salt)
}

func (s *sUser) UpdateProfile(ctx context.Context, userID uint64, in *service.ProfileInput) (*service.User, error) {
	profile, err := checkProfile(in)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, service.ErrUserNotFound
	}
	if err := s.repo.UpdateProfile(ctx, userID, profile.Avatar, profile.Sex, profile.Sign); err != nil {
		return nil, err
	}
	record.Avatar, record.Sex, record.Sign = profile.Avatar, profile.Sex, profile.Sign
	return toDomain(record), nil
}

// checkProfile validates and normalises the editable profile fields. The
// avatar is shown as an image and may be turned into a link by a front end,
// so only an absolute http(s) URL is accepted (javascript: and data: are not).
func checkProfile(in *service.ProfileInput) (*service.ProfileInput, error) {
	avatar := strings.TrimSpace(in.Avatar)
	sign := strings.TrimSpace(in.Sign)
	if in.Sex > service.MaxSex || utf8.RuneCountInString(sign) > service.MaxSignRunes {
		return nil, service.ErrInvalidArgument
	}
	if avatar != "" {
		if len(avatar) > service.MaxAvatarBytes || strings.IndexFunc(avatar, unicode.IsControl) >= 0 {
			return nil, service.ErrInvalidArgument
		}
		u, err := url.Parse(avatar)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return nil, service.ErrInvalidArgument
		}
	}
	return &service.ProfileInput{Avatar: avatar, Sex: in.Sex, Sign: sign}, nil
}

func toDomain(record *entity.UserInfo) *service.User {
	return &service.User{
		ID:        record.Id,
		Name:      record.Name,
		Avatar:    record.Avatar,
		Sex:       record.Sex,
		Status:    record.Status,
		Sign:      record.Sign,
		CreatedAt: record.CreatedAt,
	}
}
