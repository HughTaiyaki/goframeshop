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

	v1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
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

// The file service has no public methods and trusts only administrator tokens.
func TestEveryRPCNeedsAnAdminToken(t *testing.T) {
	adminSigner := newSigner(t, "goframeshop-admin")
	server := newServer(adminSigner.Verifier())
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
	client := v1.NewFileServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userToken, _, _ := newSigner(t, "goframeshop-user").Issue(1, time.Now())
	adminToken, _, _ := adminSigner.Issue(1, time.Now())
	asUser := metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+userToken)
	asAdmin := metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+adminToken)

	for name, c := range map[string]context.Context{"no token": ctx, "shop user token": asUser} {
		if _, err := client.GetFile(c, &v1.GetFileRequest{FileId: 1}); status.Code(err) != codes.Unauthenticated {
			t.Errorf("GetFile with %s: got %s", name, status.Code(err))
		}
		if _, err := client.CreateFile(c, &v1.CreateFileRequest{}); status.Code(err) != codes.Unauthenticated {
			t.Errorf("CreateFile with %s: got %s", name, status.Code(err))
		}
	}
	if _, err := client.CreateFile(asAdmin, &v1.CreateFileRequest{Storage: "s3"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("admin with bad request: got %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}
