package service

import (
	"context"
)

// RegisterInput is already validated by the transport layer. The application
// layer re-checks the rules that need domain knowledge, such as uniqueness.
type RegisterInput struct {
	Username string
	Password string
	Avatar   string
	Sex      uint
	Sign     string
}

// ProfileInput holds the editable profile fields. It is a full replacement:
// an empty avatar or sign clears it. The username and the status are not
// editable by the user.
type ProfileInput struct {
	Avatar string
	Sex    uint
	Sign   string
}

// IUser is the user application service.
type IUser interface {
	Register(ctx context.Context, in *RegisterInput) (*User, error)
	Login(ctx context.Context, username, password string) (*User, error)
	GetProfile(ctx context.Context, userID uint64) (*User, error)
	ChangePassword(ctx context.Context, userID uint64, currentPassword, newPassword string) error
	UpdateProfile(ctx context.Context, userID uint64, in *ProfileInput) (*User, error)
}
