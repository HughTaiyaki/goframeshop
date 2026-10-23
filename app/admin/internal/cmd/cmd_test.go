package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	v1 "github.com/hughtaiyaki/goframeshop/app/admin/api/admin/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
)

func newSigner(t *testing.T, issuer string) *utility.TokenSigner {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := utility.NewTokenSigner(privateKey.Seed(), issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// The admin service signs with its own key pair. A token from the user
// service, even for the same numeric id, must never pass here.
func TestAdminTokensAreSeparateFromUserTokens(t *testing.T) {
	_, adminKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	adminSigner, err := utility.NewTokenSigner(adminKey.Seed(), "goframeshop-admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	server := newServer(adminSigner)
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
	client := v1.NewAdminServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	withToken := func(token string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+token)
	}
	create := &v1.CreateAdminRequest{Name: "x", Password: "correct-horse-battery"}

	userToken, _, _ := newSigner(t, "goframeshop-user").Issue(1, time.Now())
	// Same key, wrong issuer: also refused, so the issuer check stands on its
	// own even if two deployments were ever given the same key by mistake.
	sameKeyOtherIssuer, err := utility.NewTokenSigner(adminKey.Seed(), "goframeshop-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	wrongIssuer, _, _ := sameKeyOtherIssuer.Issue(1, time.Now())
	adminToken, _, _ := adminSigner.Issue(1, time.Now())

	for name, c := range map[string]context.Context{
		"no token":           ctx,
		"shop user token":    withToken(userToken),
		"other issuer token": withToken(wrongIssuer),
		"legacy x-user-id":   metadata.AppendToOutgoingContext(ctx, "x-user-id", "1"),
	} {
		if _, err := client.CreateAdmin(c, create); status.Code(err) != codes.Unauthenticated {
			t.Errorf("%s: got %s, want %s", name, status.Code(err), codes.Unauthenticated)
		}
	}

	if _, err := client.CreateAdmin(withToken(adminToken), &v1.CreateAdminRequest{Name: "x", Password: "short"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("weak password with admin token: got %s, want %s", status.Code(err), codes.InvalidArgument)
	}
	if _, err := client.Login(ctx, &v1.LoginRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("login is public and validated: got %s", status.Code(err))
	}
}
