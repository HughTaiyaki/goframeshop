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

	positionv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"
	rotationv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
)

func signer(t *testing.T, issuer string) *utility.TokenSigner {
	t.Helper()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	s, err := utility.NewTokenSigner(key.Seed(), issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Lists are public; writes need an administrator token. Every case is decided
// before a handler runs, so no database is needed.
func TestReadsArePublicAndWritesNeedAnAdmin(t *testing.T) {
	admin := signer(t, "goframeshop-admin")
	server := newServer(admin.Verifier(), nil, nil)
	listener := bufconn.Listen(1 << 20)
	done := make(chan error, 1)
	go func() { done <- server.Server.Serve(listener) }()
	t.Cleanup(func() { server.Server.Stop(); _ = listener.Close(); <-done })
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	rotations, positions := rotationv1.NewRotationServiceClient(conn), positionv1.NewPositionServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := rotations.ListRotations(ctx, &rotationv1.ListRotationsRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("ListRotations without token: %s", status.Code(err))
	}
	if _, err := positions.ListPositions(ctx, &positionv1.ListPositionsRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("ListPositions without token: %s", status.Code(err))
	}

	adminToken, _, _ := admin.Issue(1, time.Now())
	shopToken, _, _ := signer(t, "goframeshop-user").Issue(1, time.Now())
	with := func(token string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+token)
	}
	writes := map[string]func(context.Context) error{
		"CreateRotation": func(c context.Context) error {
			_, err := rotations.CreateRotation(c, &rotationv1.CreateRotationRequest{})
			return err
		},
		"UpdateRotation": func(c context.Context) error {
			_, err := rotations.UpdateRotation(c, &rotationv1.UpdateRotationRequest{})
			return err
		},
		"DeleteRotation": func(c context.Context) error {
			_, err := rotations.DeleteRotation(c, &rotationv1.DeleteRotationRequest{})
			return err
		},
		"CreatePosition": func(c context.Context) error {
			_, err := positions.CreatePosition(c, &positionv1.CreatePositionRequest{})
			return err
		},
		"UpdatePosition": func(c context.Context) error {
			_, err := positions.UpdatePosition(c, &positionv1.UpdatePositionRequest{})
			return err
		},
		"DeletePosition": func(c context.Context) error {
			_, err := positions.DeletePosition(c, &positionv1.DeletePositionRequest{})
			return err
		},
	}
	for name, write := range writes {
		if got := status.Code(write(ctx)); got != codes.Unauthenticated {
			t.Errorf("%s without token: %s", name, got)
		}
		if got := status.Code(write(with(shopToken))); got != codes.Unauthenticated {
			t.Errorf("%s with a shop token: %s", name, got)
		}
		if got := status.Code(write(with(adminToken))); got != codes.InvalidArgument {
			t.Errorf("%s as admin: %s, want InvalidArgument (past auth)", name, got)
		}
	}
}
