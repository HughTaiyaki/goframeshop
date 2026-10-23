// Package controller holds helpers shared by the transport layer handlers.
package controller

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// StatusError translates an application error into a gRPC status. Errors
// from other services keep the codes that mean the same to our caller and
// lose their text; anything else becomes a fixed Internal message.
func StatusError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, service.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "already exists")
	case errors.Is(err, service.ErrIdempotencyConflict):
		return status.Error(codes.AlreadyExists, "request id reused with different parameters")
	case errors.Is(err, service.ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, "not allowed in the current state")
	case errors.Is(err, service.ErrGoodsUnavailable):
		return status.Error(codes.FailedPrecondition, "some goods are unavailable")
	case errors.Is(err, service.ErrBadNotice):
		return status.Error(codes.Unauthenticated, "payment notification rejected")
	case errors.Is(err, service.ErrPaymentMismatch):
		return status.Error(codes.FailedPrecondition, "payment does not match the order")
	}
	switch code := utility.RPCCode(err); code {
	case codes.Unauthenticated, codes.Unavailable, codes.DeadlineExceeded:
		return status.Error(code, code.String())
	}
	g.Log().Errorf(ctx, "unexpected error: %v", err)
	return status.Error(codes.Internal, "internal error")
}

// CallerID returns the verified caller. The interceptor already refused any
// call without a valid token of the right role, so a missing id here is a
// wiring bug and fails closed.
func CallerID(ctx context.Context) (uint64, error) {
	id, ok := utility.UserIDFromContext(ctx)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "caller identity is missing")
	}
	return id, nil
}

func Timestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}
