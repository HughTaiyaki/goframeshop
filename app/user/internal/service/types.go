// Package service defines the application contracts of the user service.
//
// Controllers depend only on these interfaces, never on the database, so the
// transport layer and the persistence layer can change independently.
package service

import (
	"errors"

	"github.com/gogf/gf/v2/os/gtime"
)

// Sentinel errors. The controller layer maps them to gRPC status codes, so no
// database error ever reaches a caller.
var (
	ErrNoIdentity        = errors.New("caller identity is missing")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("username already taken")
	ErrInvalidCredential = errors.New("invalid username or password")
	ErrUserBlocked       = errors.New("user is blocked")
	ErrConsigneeNotFound = errors.New("address not found")
	ErrInvalidArgument   = errors.New("invalid argument")
)

// Limits of the editable profile fields. They match the column sizes.
const (
	MaxAvatarBytes = 512
	MaxSignRunes   = 255
	MaxSex         = 2
)

// User is the domain view of a user. It deliberately omits password_hash,
// password_salt and secret_answer_hash so secrets cannot leak by accident.
type User struct {
	ID        uint64
	Name      string
	Avatar    string
	Sex       uint
	Status    uint
	Sign      string
	CreatedAt *gtime.Time
}

// Consignee is the domain view of a shipping address. DeletedAt is not part of
// the domain type: soft deletion is a persistence concern.
type Consignee struct {
	ID        uint64
	UserID    uint64
	IsDefault bool
	Name      string
	Phone     string
	Province  string
	City      string
	Town      string
	Street    string
	Detail    string
	CreatedAt *gtime.Time
	UpdatedAt *gtime.Time
}
