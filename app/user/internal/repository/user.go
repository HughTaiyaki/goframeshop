// Package repository contains the database-backed implementations of the
// repository interfaces declared in internal/service.
//
// This is the only layer that is allowed to touch the generated DAO.
package repository

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

type userRepository struct{}

// NewUser returns the MySQL-backed user repository.
func NewUser() service.UserRepository {
	return &userRepository{}
}

func (r *userRepository) Create(ctx context.Context, u *entity.UserInfo) (uint64, error) {
	id, err := dao.UserInfo.Ctx(ctx).Data(u).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// FindByName returns nil (with a nil error) when no active user matches, so the
// caller can distinguish "not found" from "database failure".
func (r *userRepository) FindByName(ctx context.Context, name string) (*entity.UserInfo, error) {
	columns := dao.UserInfo.Columns()
	one, err := dao.UserInfo.Ctx(ctx).
		Where(columns.Name, name).
		WhereNull(columns.DeletedAt).
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var user entity.UserInfo
	if err := one.Struct(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint64) (*entity.UserInfo, error) {
	columns := dao.UserInfo.Columns()
	one, err := dao.UserInfo.Ctx(ctx).
		Where(columns.Id, id).
		WhereNull(columns.DeletedAt).
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var user entity.UserInfo
	if err := one.Struct(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) UpdateProfile(ctx context.Context, id uint64, avatar string, sex uint, sign string) error {
	columns := dao.UserInfo.Columns()
	_, err := dao.UserInfo.Ctx(ctx).
		Data(gdb.Map{
			columns.Avatar: avatar,
			columns.Sex:    sex,
			columns.Sign:   sign,
		}).
		Where(columns.Id, id).
		WhereNull(columns.DeletedAt).
		Update()
	return err
}

func (r *userRepository) UpdateCredential(ctx context.Context, id uint64, hash string, salt []byte) error {
	_, err := dao.UserInfo.Ctx(ctx).
		Data(gdb.Map{
			dao.UserInfo.Columns().PasswordHash: hash,
			dao.UserInfo.Columns().PasswordSalt: salt,
		}).
		Where(dao.UserInfo.Columns().Id, id).
		Update()
	return err
}
