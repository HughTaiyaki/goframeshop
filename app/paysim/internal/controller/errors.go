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

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

func StatusError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, service.ErrConflict):
		return status.Error(codes.AlreadyExists, "id already used with different parameters")
	case errors.Is(err, service.ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, "not allowed in the current state")
	case errors.Is(err, service.ErrExpired):
		return status.Error(codes.FailedPrecondition, "payment expired")
	}
	g.Log().Errorf(ctx, "unexpected error: %v", err)
	return status.Error(codes.Internal, "internal error")
}

func Timestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}
