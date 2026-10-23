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

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

func StatusError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	}
	switch code := utility.RPCCode(err); code {
	case codes.Unauthenticated, codes.Unavailable, codes.DeadlineExceeded:
		return status.Error(code, code.String())
	}
	g.Log().Errorf(ctx, "unexpected error: %v", err)
	return status.Error(codes.Internal, "internal error")
}

func Timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
