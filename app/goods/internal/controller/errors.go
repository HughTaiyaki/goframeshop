// Package controller holds helpers shared by the transport layer handlers.
package controller

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// StatusError translates an application error into a gRPC status. Unknown
// errors collapse into a fixed Internal message.
func StatusError(err error) error {
	if err == nil {
		return nil
	}
	// An error from a downstream call (the file service). Pass on the codes
	// that mean the same thing to our caller, but never the downstream text.
	switch code := utility.RPCCode(err); code {
	case codes.Unauthenticated, codes.Unavailable, codes.DeadlineExceeded:
		return status.Error(code, code.String())
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
	case errors.Is(err, service.ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, "resource is still in use")
	case errors.Is(err, service.ErrInsufficientStock):
		return status.Error(codes.FailedPrecondition, "not enough available stock")
	case errors.Is(err, service.ErrCartLimit):
		return status.Error(codes.FailedPrecondition, "cart limit reached")
	case errors.Is(err, service.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, "operation id reused with different parameters")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// IsAdmin reports whether the caller presented a valid administrator token.
// A service token (the order service) is not an administrator: it sees the
// public catalogue only.
func IsAdmin(ctx context.Context) bool {
	role, ok := utility.RoleFromContext(ctx)
	return ok && role == utility.RoleAdmin
}

// Timestamp converts a domain time; the zero time becomes nil, not 1970.
func Timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
