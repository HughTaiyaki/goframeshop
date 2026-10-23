package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func newTestSigner(t *testing.T) *utility.TokenSigner {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := utility.NewTokenSigner(privateKey.Seed(), "goframeshop-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// This test drives the real gRPC stack over an in-memory connection, so the
// auth and validation interceptors, the server chain and the status mapping
// are all exercised. It never needs MySQL: every case either fails before the
// handler, or only checks that the call got past authentication.
func TestGRPCValidationAndAuthGate(t *testing.T) {
	signer := newTestSigner(t)
	server := newServer(signer)
	listener := bufconn.Listen(1024 * 1024)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Server.Serve(listener) }()
	t.Cleanup(func() {
		server.Server.Stop()
		_ = listener.Close()
		<-serveDone
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	userClient := userv1.NewUserServiceClient(conn)
	consigneeClient := consigneev1.NewConsigneeServiceClient(conn)

	token, _, err := signer.Issue(42, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	authed := metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+token)

	t.Run("public methods are validated without a token", func(t *testing.T) {
		if _, err := userClient.RegisterUser(ctx, &userv1.RegisterRequest{}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("empty register: got %s, want %s", status.Code(err), codes.InvalidArgument)
		}
		if _, err := userClient.Login(ctx, &userv1.LoginRequest{}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("empty login: got %s, want %s", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("protected methods are validated after authentication", func(t *testing.T) {
		if _, err := consigneeClient.ListConsignees(authed, &consigneev1.ListConsigneesRequest{Page: 0, Size: 10}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("zero page: got %s, want %s", status.Code(err), codes.InvalidArgument)
		}
		if _, err := consigneeClient.DeleteConsignee(authed, &consigneev1.DeleteConsigneeRequest{}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("missing id: got %s, want %s", status.Code(err), codes.InvalidArgument)
		}
		if _, err := userClient.UpdateProfile(authed, &userv1.UpdateProfileRequest{Sex: 3}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("sex 3: got %s, want %s", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("unauthenticated callers learn nothing about validation", func(t *testing.T) {
		_, err := consigneeClient.DeleteConsignee(ctx, &consigneev1.DeleteConsigneeRequest{})
		if got := status.Code(err); got != codes.Unauthenticated {
			t.Fatalf("got %s, want %s", got, codes.Unauthenticated)
		}
	})

	t.Run("every protected RPC rejects a missing, forged or legacy identity", func(t *testing.T) {
		forged, _, err := newTestSigner(t).Issue(42, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		callers := map[string]context.Context{
			"no token":       ctx,
			"forged token":   metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+forged),
			"legacy user id": metadata.AppendToOutgoingContext(ctx, "x-user-id", "42"),
		}
		rpcs := map[string]func(context.Context) error{
			"GetProfile": func(c context.Context) error {
				_, err := userClient.GetProfile(c, &userv1.GetProfileRequest{})
				return err
			},
			"ChangePassword": func(c context.Context) error {
				_, err := userClient.ChangePassword(c, &userv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: "long-enough"})
				return err
			},
			"UpdateProfile": func(c context.Context) error {
				_, err := userClient.UpdateProfile(c, &userv1.UpdateProfileRequest{})
				return err
			},
			"ListConsignees": func(c context.Context) error {
				_, err := consigneeClient.ListConsignees(c, &consigneev1.ListConsigneesRequest{Page: 1, Size: 10})
				return err
			},
		}
		for callerName, callerCtx := range callers {
			for rpcName, rpc := range rpcs {
				if got := status.Code(rpc(callerCtx)); got != codes.Unauthenticated {
					t.Errorf("%s with %s: got %s, want %s", rpcName, callerName, got, codes.Unauthenticated)
				}
			}
		}
	})

	t.Run("a valid token gets past authentication", func(t *testing.T) {
		_, err := consigneeClient.ListConsignees(authed, &consigneev1.ListConsigneesRequest{Page: 1, Size: 10})
		if got := status.Code(err); got == codes.Unauthenticated {
			t.Fatal("a valid token was rejected")
		}
	})
}
