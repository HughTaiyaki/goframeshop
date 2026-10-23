// Package controller holds helpers shared by the transport layer handlers.
package controller

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

// StatusError translates an application error into a gRPC status.
//
// Unknown errors collapse into codes.Internal with a fixed message, so a driver
// error, a table name or a constraint violation never reaches the caller.
func StatusError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, service.ErrNoIdentity):
		return status.Error(codes.Unauthenticated, "caller identity is missing")
	case errors.Is(err, service.ErrInvalidCredential):
		return status.Error(codes.Unauthenticated, "invalid username or password")
	case errors.Is(err, service.ErrUserBlocked):
		return status.Error(codes.PermissionDenied, "user is blocked")
	case errors.Is(err, service.ErrUserExists):
		return status.Error(codes.AlreadyExists, "username already taken")
	case errors.Is(err, service.ErrUserNotFound), errors.Is(err, service.ErrConsigneeNotFound):
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
