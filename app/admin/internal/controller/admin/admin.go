// Package admin is the gRPC transport layer for administrators.
package admin

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/hughtaiyaki/goframeshop/app/admin/api/admin/v1"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedAdminServiceServer
	admins service.IAdmin
	tokens *utility.TokenSigner
	now    func() time.Time
}

// Register wires the controller. The signer is passed in so a missing signing
// key fails at startup, not on the first login.
func Register(s *grpcx.GrpcServer, tokens *utility.TokenSigner) {
	v1.RegisterAdminServiceServer(s.Server, NewController(logic.Admin(), tokens))
}

// NewController exists so tests can supply a fake application service.
func NewController(admins service.IAdmin, tokens *utility.TokenSigner) *Controller {
	return &Controller{admins: admins, tokens: tokens, now: time.Now}
}

func (c *Controller) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	admin, err := c.admins.Login(ctx, req.Name, req.Password)
	if err != nil {
		return nil, statusError(err)
	}
	now := c.now()
	token, expiresAt, err := c.tokens.Issue(admin.ID, now)
	if err != nil {
		return nil, statusError(err)
	}
	return &v1.LoginResponse{
		TokenType:        utility.BearerScheme,
		AccessToken:      token,
		ExpiresInSeconds: uint32(expiresAt.Sub(now).Seconds()),
		AdminId:          admin.ID,
		Name:             admin.Name,
		IsSuper:          admin.IsSuper,
	}, nil
}

func (c *Controller) CreateAdmin(ctx context.Context, req *v1.CreateAdminRequest) (*v1.CreateAdminResponse, error) {
	callerID, ok := utility.UserIDFromContext(ctx)
	if !ok {
		return nil, statusError(service.ErrNoIdentity)
	}
	id, err := c.admins.Create(ctx, callerID, &service.CreateInput{
		Name:     req.Name,
		Password: req.Password,
		IsSuper:  req.IsSuper,
	})
	if err != nil {
		return nil, statusError(err)
	}
	return &v1.CreateAdminResponse{AdminId: id}, nil
}

// statusError translates an application error into a gRPC status. Unknown
// errors collapse into a fixed Internal message so nothing from the database
// reaches the caller.
func statusError(err error) error {
	switch {
	case errors.Is(err, service.ErrNoIdentity):
		return status.Error(codes.Unauthenticated, "caller identity is missing")
	case errors.Is(err, service.ErrInvalidCredential):
		return status.Error(codes.Unauthenticated, "invalid name or password")
	case errors.Is(err, service.ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, "only a super administrator may do this")
	case errors.Is(err, service.ErrAdminExists):
		return status.Error(codes.AlreadyExists, "administrator name already taken")
	case errors.Is(err, service.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, "invalid argument")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
