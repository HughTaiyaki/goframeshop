package service

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
)

// UserRepository is the persistence seam for users. It speaks in database
// entities; converting them to domain types is the job of the logic layer.
type UserRepository interface {
	Create(ctx context.Context, u *entity.UserInfo) (uint64, error)
	FindByName(ctx context.Context, name string) (*entity.UserInfo, error)
	FindByID(ctx context.Context, id uint64) (*entity.UserInfo, error)
	UpdateCredential(ctx context.Context, id uint64, hash string, salt []byte) error
	UpdateProfile(ctx context.Context, id uint64, avatar string, sex uint, sign string) error
}

// ConsigneeRepository is the persistence seam for addresses.
//
// UpdateAsDefault exists because "make this address the default" is two writes
// (clear the old default, set the new one) that must succeed or fail together.
// Keeping the transaction inside the repository means callers cannot forget it.
type ConsigneeRepository interface {
	Create(ctx context.Context, c *entity.ConsigneeInfo) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.ConsigneeInfo, error)
	ListByUser(ctx context.Context, userID uint64, page, size int) ([]*entity.ConsigneeInfo, error)
	CountByUser(ctx context.Context, userID uint64) (int, error)
	Update(ctx context.Context, c *entity.ConsigneeInfo) error
	UpdateAsDefault(ctx context.Context, c *entity.ConsigneeInfo) error
	SoftDelete(ctx context.Context, userID, id uint64) error
}
