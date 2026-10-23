// Package service defines the application contracts of the interaction service.
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
	ErrNoIdentity      = errors.New("caller identity is missing")
	ErrNotFound        = errors.New("resource not found")
	ErrAlreadyExists   = errors.New("resource already exists")
	ErrInvalidArgument = errors.New("invalid argument")
)

// Object types a user can praise, collect or comment on.
const (
	ObjectTypeGoods   uint32 = 1
	ObjectTypeArticle uint32 = 2
)

// ValidObjectType reports whether t is a known object type.
func ValidObjectType(t uint32) bool {
	return t == ObjectTypeGoods || t == ObjectTypeArticle
}

// Mark is one user's praise or collection of one object. Praise and collection
// have exactly the same shape and rules, so they share this type and one
// application service; only the table behind the repository differs.
type Mark struct {
	ID         uint64
	UserID     uint64
	ObjectType uint32
	ObjectID   uint64
	CreatedAt  *gtime.Time
}

// Comment is a top-level comment (ParentID 0) or a reply.
type Comment struct {
	ID         uint64
	ParentID   uint64
	UserID     uint64
	ObjectType uint32
	ObjectID   uint64
	Content    string
	CreatedAt  *gtime.Time
}
