package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"
	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
)

func newSigner(t *testing.T, issuer string) *utility.TokenSigner {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := utility.NewTokenSigner(key.Seed(), issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// Every case here is decided by the interceptors before a handler runs, so
// the test needs neither MySQL nor Redis: an InvalidArgument proves the call
// got past authentication, an Unauthenticated proves it did not.
func TestReadsArePublicAndWritesNeedAnAdmin(t *testing.T) {
	admin := newSigner(t, "goframeshop-admin")
	orderSigner := newSigner(t, "goframeshop-order")
	server := newServer(admin.Verifier(), orderSigner.Verifier())
	listener := bufconn.Listen(1024 * 1024)
	done := make(chan error, 1)
	go func() { done <- server.Server.Serve(listener) }()
	t.Cleanup(func() { server.Server.Stop(); _ = listener.Close(); <-done })
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	goods := goodsv1.NewGoodsServiceClient(conn)
	categories := categoryv1.NewCategoryServiceClient(conn)
	inventory := inventoryv1.NewInventoryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	with := func(token string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+token)
	}
	adminToken, _, _ := admin.Issue(1, time.Now())
	shopToken, _, _ := newSigner(t, "goframeshop-user").Issue(1, time.Now())
	serviceToken, _, _ := orderSigner.Issue(1, time.Now())

	t.Run("reads need no token", func(t *testing.T) {
		if _, err := goods.GetGoods(ctx, &goodsv1.GetGoodsRequest{}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("GetGoods: %s", status.Code(err))
		}
		if _, err := goods.ListGoods(ctx, &goodsv1.ListGoodsRequest{Page: 0, Size: 10}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("ListGoods: %s", status.Code(err))
		}
		if _, err := categories.ListCategories(ctx, &categoryv1.ListCategoriesRequest{Page: 1, Size: 0}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("ListCategories: %s", status.Code(err))
		}
	})

	writes := map[string]func(context.Context) error{
		"CreateGoods": func(c context.Context) error {
			_, err := goods.CreateGoods(c, &goodsv1.CreateGoodsRequest{Goods: &goodsv1.GoodsInput{}})
			return err
		},
		"UpdateGoods": func(c context.Context) error {
			_, err := goods.UpdateGoods(c, &goodsv1.UpdateGoodsRequest{Goods: &goodsv1.GoodsInput{}})
			return err
		},
		"DeleteGoods": func(c context.Context) error {
			_, err := goods.DeleteGoods(c, &goodsv1.DeleteGoodsRequest{})
			return err
		},
		"AddGoodsImage": func(c context.Context) error {
			_, err := goods.AddGoodsImage(c, &goodsv1.AddGoodsImageRequest{})
			return err
		},
		"RemoveGoodsImage": func(c context.Context) error {
			_, err := goods.RemoveGoodsImage(c, &goodsv1.RemoveGoodsImageRequest{})
			return err
		},
		"CreateCategory": func(c context.Context) error {
			_, err := categories.CreateCategory(c, &categoryv1.CreateCategoryRequest{})
			return err
		},
		"UpdateCategory": func(c context.Context) error {
			_, err := categories.UpdateCategory(c, &categoryv1.UpdateCategoryRequest{})
			return err
		},
		"DeleteCategory": func(c context.Context) error {
			_, err := categories.DeleteCategory(c, &categoryv1.DeleteCategoryRequest{})
			return err
		},
		"SetGoodsShelf": func(c context.Context) error {
			_, err := goods.SetGoodsShelf(c, &goodsv1.SetGoodsShelfRequest{})
			return err
		},
		"AdjustStock": func(c context.Context) error {
			_, err := goods.AdjustStock(c, &goodsv1.AdjustStockRequest{})
			return err
		},
		"ReserveInventory": func(c context.Context) error {
			_, err := inventory.ReserveInventory(c, &inventoryv1.ReserveInventoryRequest{})
			return err
		},
		"CommitInventory": func(c context.Context) error {
			_, err := inventory.CommitInventory(c, &inventoryv1.CommitInventoryRequest{})
			return err
		},
		"ReleaseInventory": func(c context.Context) error {
			_, err := inventory.ReleaseInventory(c, &inventoryv1.ReleaseInventoryRequest{})
			return err
		},
		"QueryInventoryOperation": func(c context.Context) error {
			_, err := inventory.QueryInventoryOperation(c, &inventoryv1.QueryInventoryOperationRequest{})
			return err
		},
	}
	for name, write := range writes {
		for caller, c := range map[string]context.Context{"no token": ctx, "shop user token": with(shopToken)} {
			if got := status.Code(write(c)); got != codes.Unauthenticated {
				t.Errorf("%s with %s: got %s, want Unauthenticated", name, caller, got)
			}
		}
		// The request is empty, so validation rejects it: proof that an
		// administrator token got through authentication.
		if got := status.Code(write(with(adminToken))); got != codes.InvalidArgument {
			t.Errorf("%s as admin: got %s, want InvalidArgument", name, got)
		}
	}

	t.Run("the order service may use inventory and nothing else", func(t *testing.T) {
		for name, write := range writes {
			got := status.Code(write(with(serviceToken)))
			inventoryRPC := strings.HasSuffix(name, "Inventory") || name == "QueryInventoryOperation"
			switch {
			case inventoryRPC && got != codes.InvalidArgument:
				t.Errorf("%s with a service token: got %s, want InvalidArgument (past auth)", name, got)
			case !inventoryRPC && got != codes.PermissionDenied:
				t.Errorf("%s with a service token: got %s, want PermissionDenied", name, got)
			}
		}
	})

	t.Run("nested goods fields are validated", func(t *testing.T) {
		_, err := goods.CreateGoods(with(adminToken), &goodsv1.CreateGoodsRequest{Goods: &goodsv1.GoodsInput{
			Name: "TV", PriceCents: 0, Level1CategoryId: 1,
		}})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("zero price inside GoodsInput: got %s", status.Code(err))
		}
	})
}
