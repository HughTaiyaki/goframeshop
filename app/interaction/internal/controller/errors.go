// Package controller holds helpers shared by the transport layer handlers.
package controller

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
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
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, service.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "resource already exists")
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// RequireUserID reads the identity that the auth interceptor verified. A
// protected handler reached without it is a wiring bug, and it fails closed.
func RequireUserID(ctx context.Context) (uint64, error) {
	userID, ok := utility.UserIDFromContext(ctx)
	if !ok {
		return 0, StatusError(service.ErrNoIdentity)
	}
	return userID, nil
}
