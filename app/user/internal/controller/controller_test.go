package controller_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/controller/consignee"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/controller/user"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Every RPC that acts on "my" data must refuse an unauthenticated call before
// touching the database. These controllers have no repository wired in, so
// reaching the database would panic here: a passing test proves the identity
// check really does come first.
func TestOwnDataRPCsRequireCallerIdentity(t *testing.T) {
	ctx := context.Background()
	userController := &user.Controller{}
	consigneeController := &consignee.Controller{}

	tests := []struct {
		name string
		call func() error
	}{
		{"GetProfile", func() error { _, err := userController.GetProfile(ctx, &userv1.GetProfileRequest{}); return err }},
		{"ChangePassword", func() error {
			_, err := userController.ChangePassword(ctx, &userv1.ChangePasswordRequest{})
			return err
		}},
		{"UpdateProfile", func() error {
			_, err := userController.UpdateProfile(ctx, &userv1.UpdateProfileRequest{})
			return err
		}},
		{"ListConsignees", func() error {
			_, err := consigneeController.ListConsignees(ctx, &consigneev1.ListConsigneesRequest{Page: 1, Size: 10})
			return err
		}},
		{"CreateConsignee", func() error {
			_, err := consigneeController.CreateConsignee(ctx, &consigneev1.CreateConsigneeRequest{Name: "x"})
			return err
		}},
		{"UpdateConsignee", func() error {
			_, err := consigneeController.UpdateConsignee(ctx, &consigneev1.UpdateConsigneeRequest{ConsigneeId: 1})
			return err
		}},
		{"DeleteConsignee", func() error {
			_, err := consigneeController.DeleteConsignee(ctx, &consigneev1.DeleteConsigneeRequest{ConsigneeId: 1})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status.Code(tt.call()); got != codes.Unauthenticated {
				t.Fatalf("status code = %s, want %s", got, codes.Unauthenticated)
			}
		})
	}
}

// fakeUsers is just enough of service.IUser to drive Login.
type fakeUsers struct {
	service.IUser
	account *service.User
	err     error
}

func (f *fakeUsers) Login(context.Context, string, string) (*service.User, error) {
	return f.account, f.err
}

func TestLoginIssuesAVerifiableToken(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := utility.NewTokenSigner(privateKey.Seed(), "goframeshop-user", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c := user.NewController(&fakeUsers{account: &service.User{ID: 7, Name: "alice"}}, signer)

	res, err := c.Login(context.Background(), &userv1.LoginRequest{Username: "alice", Password: "whatever"})
	if err != nil {
		t.Fatal(err)
	}
	if res.TokenType != "Bearer" || res.ExpiresInSeconds != 7200 || res.Profile.GetUserId() != 7 {
		t.Fatalf("unexpected response: type=%q expires=%d user=%d", res.TokenType, res.ExpiresInSeconds, res.Profile.GetUserId())
	}
	userID, err := signer.Verifier().Parse(res.AccessToken, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if userID != 7 {
		t.Fatalf("token subject = %d, want 7", userID)
	}
}

func TestLoginFailureIssuesNoToken(t *testing.T) {
	c := user.NewController(&fakeUsers{err: service.ErrInvalidCredential}, nil)

	res, err := c.Login(context.Background(), &userv1.LoginRequest{Username: "alice", Password: "wrong"})
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("code = %s, want %s", got, codes.Unauthenticated)
	}
	if res != nil {
		t.Fatal("a failed login returned a response")
	}
}
