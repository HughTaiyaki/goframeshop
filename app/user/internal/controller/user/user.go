// Package user is the gRPC transport layer for the user API.
//
// Handlers stay deliberately thin: read the request, resolve the caller, call
// the application service, map the result. No SQL and no business rules here.
package user

import (
	"context"
	"time"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedUserServiceServer
	users  service.IUser
	tokens *utility.TokenSigner
	now    func() time.Time
}

// Register wires the controller. The signer is passed in rather than loaded
// here so a missing signing key fails at startup, not on the first login.
func Register(s *grpcx.GrpcServer, tokens *utility.TokenSigner) {
	v1.RegisterUserServiceServer(s.Server, NewController(logic.User(), tokens))
}

// NewController exists so tests can supply a fake application service.
func NewController(users service.IUser, tokens *utility.TokenSigner) *Controller {
	return &Controller{users: users, tokens: tokens, now: time.Now}
}

func (c *Controller) RegisterUser(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterResponse, error) {
	created, err := c.users.Register(ctx, &service.RegisterInput{
		Username: req.Username,
		Password: req.Password,
		Avatar:   req.Avatar,
		Sex:      uint(req.Sex),
		Sign:     req.Sign,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.RegisterResponse{UserId: created.ID}, nil
}

// Login verifies the password and issues a short-lived access token. The token
// is the only proof of identity the protected RPCs accept.
func (c *Controller) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	account, err := c.users.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	now := c.now()
	token, expiresAt, err := c.tokens.Issue(account.ID, now)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.LoginResponse{
		TokenType:        utility.BearerScheme,
		AccessToken:      token,
		ExpiresInSeconds: uint32(expiresAt.Sub(now).Seconds()),
		Profile:          toProfile(account),
	}, nil
}

func (c *Controller) GetProfile(ctx context.Context, req *v1.GetProfileRequest) (*v1.GetProfileResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	account, err := c.users.GetProfile(ctx, userID)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.GetProfileResponse{Profile: toProfile(account)}, nil
}

func (c *Controller) ChangePassword(ctx context.Context, req *v1.ChangePasswordRequest) (*v1.ChangePasswordResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.users.ChangePassword(ctx, userID, req.CurrentPassword, req.NewPassword); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.ChangePasswordResponse{}, nil
}

func (c *Controller) UpdateProfile(ctx context.Context, req *v1.UpdateProfileRequest) (*v1.UpdateProfileResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	account, err := c.users.UpdateProfile(ctx, userID, &service.ProfileInput{
		Avatar: req.Avatar,
		Sex:    uint(req.Sex),
		Sign:   req.Sign,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.UpdateProfileResponse{Profile: toProfile(account)}, nil
}

// requireUserID reads the identity that the auth interceptor verified. A
// handler reached without it is a wiring bug, and it fails closed.
func requireUserID(ctx context.Context) (uint64, error) {
	userID, ok := utility.UserIDFromContext(ctx)
	if !ok {
		return 0, controller.StatusError(service.ErrNoIdentity)
	}
	return userID, nil
}

func toProfile(account *service.User) *v1.UserProfile {
	return &v1.UserProfile{
		UserId:    account.ID,
		Username:  account.Name,
		Avatar:    account.Avatar,
		Sex:       uint32(account.Sex),
		Sign:      account.Sign,
		Status:    uint32(account.Status),
		CreatedAt: utility.SafeConvertTime(account.CreatedAt),
	}
}
