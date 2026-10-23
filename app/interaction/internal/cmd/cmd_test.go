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

	collectionv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/collection/v1"
	commentv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"
	praisev1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/praise/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
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

// This test drives the real gRPC stack over an in-memory connection. The
// server only gets the public half of the key pair, exactly as in production.
// It never needs MySQL: every case either fails before the handler, or only
// checks that the call got past authentication.
func TestGRPCValidationAndAuthGate(t *testing.T) {
	signer := newTestSigner(t)
	server := newServer(signer.Verifier())
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
	praises := praisev1.NewPraiseServiceClient(conn)
	collections := collectionv1.NewCollectionServiceClient(conn)
	comments := commentv1.NewCommentServiceClient(conn)

	token, _, err := signer.Issue(42, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	authed := metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+token)

	t.Run("listing comments is public and validated", func(t *testing.T) {
		_, err := comments.ListComments(ctx, &commentv1.ListCommentsRequest{ObjectType: 9, ObjectId: 1, Page: 1, Size: 10})
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Fatalf("got %s, want %s", got, codes.InvalidArgument)
		}
		_, err = comments.ListComments(ctx, &commentv1.ListCommentsRequest{ObjectType: 1, ObjectId: 1, Page: 1, Size: 10})
		if got := status.Code(err); got == codes.Unauthenticated {
			t.Fatal("a public method demanded a token")
		}
	})

	t.Run("protected methods are validated after authentication", func(t *testing.T) {
		checks := map[string]error{
			"praise with unknown type": func() error {
				_, err := praises.CreatePraise(authed, &praisev1.CreatePraiseRequest{ObjectType: 3, ObjectId: 1})
				return err
			}(),
			"collection without object": func() error {
				_, err := collections.CreateCollection(authed, &collectionv1.CreateCollectionRequest{ObjectType: 1})
				return err
			}(),
			"comment too long": func() error {
				long := make([]rune, 256)
				for i := range long {
					long[i] = '好'
				}
				_, err := comments.CreateComment(authed, &commentv1.CreateCommentRequest{ObjectType: 1, ObjectId: 1, Content: string(long)})
				return err
			}(),
		}
		for name, err := range checks {
			if got := status.Code(err); got != codes.InvalidArgument {
				t.Errorf("%s: got %s, want %s", name, got, codes.InvalidArgument)
			}
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
			"ListPraises": func(c context.Context) error {
				_, err := praises.ListPraises(c, &praisev1.ListPraisesRequest{ObjectType: 1, Page: 1, Size: 10})
				return err
			},
			"CreatePraise": func(c context.Context) error {
				_, err := praises.CreatePraise(c, &praisev1.CreatePraiseRequest{ObjectType: 1, ObjectId: 1})
				return err
			},
			"DeletePraise": func(c context.Context) error {
				_, err := praises.DeletePraise(c, &praisev1.DeletePraiseRequest{PraiseId: 1})
				return err
			},
			"ListCollections": func(c context.Context) error {
				_, err := collections.ListCollections(c, &collectionv1.ListCollectionsRequest{ObjectType: 1, Page: 1, Size: 10})
				return err
			},
			"CreateCollection": func(c context.Context) error {
				_, err := collections.CreateCollection(c, &collectionv1.CreateCollectionRequest{ObjectType: 1, ObjectId: 1})
				return err
			},
			"DeleteCollection": func(c context.Context) error {
				_, err := collections.DeleteCollection(c, &collectionv1.DeleteCollectionRequest{CollectionId: 1})
				return err
			},
			"CreateComment": func(c context.Context) error {
				_, err := comments.CreateComment(c, &commentv1.CreateCommentRequest{ObjectType: 1, ObjectId: 1, Content: "x"})
				return err
			},
			"DeleteComment": func(c context.Context) error {
				_, err := comments.DeleteComment(c, &commentv1.DeleteCommentRequest{CommentId: 1})
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
		_, err := praises.ListPraises(authed, &praisev1.ListPraisesRequest{ObjectType: 1, Page: 1, Size: 10})
		if got := status.Code(err); got == codes.Unauthenticated {
			t.Fatal("a valid token was rejected")
		}
	})
}
