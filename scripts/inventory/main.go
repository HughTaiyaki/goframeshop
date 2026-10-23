// Command inventory calls the goods service's InventoryService directly, for
// local demos and debugging. No gateway exposes these RPCs: in the real flow
// the order service calls them.
//
//	ADMIN_TOKEN=... go run ./scripts/inventory reserve -order o1 -op reserve-o1 2:1 3:2
//	ADMIN_TOKEN=... go run ./scripts/inventory commit  -order o1 -op commit-o1
//	ADMIN_TOKEN=... go run ./scripts/inventory release -order o1 -op release-o1 2:1 3:2
//	ADMIN_TOKEN=... go run ./scripts/inventory query   -op reserve-o1
//
// Items are goods_id:quantity. -op defaults to "<command>-<order>", which is
// how the order service derives stable operation ids.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	addr := flags.String("addr", "127.0.0.1:31005", "goods service gRPC address")
	order := flags.String("order", "", "order id")
	op := flags.String("op", "", "operation id (default <command>-<order>)")
	_ = flags.Parse(os.Args[2:])
	if *op == "" {
		*op = command + "-" + *order
	}
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		fail("set ADMIN_TOKEN to an administrator access token")
	}

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fail(err.Error())
	}
	defer conn.Close()
	client := inventoryv1.NewInventoryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

	var res proto.Message
	switch command {
	case "reserve":
		res, err = client.ReserveInventory(ctx, &inventoryv1.ReserveInventoryRequest{OperationId: *op, OrderId: *order, Items: items(flags.Args())})
	case "commit":
		res, err = client.CommitInventory(ctx, &inventoryv1.CommitInventoryRequest{OperationId: *op, OrderId: *order})
	case "release":
		res, err = client.ReleaseInventory(ctx, &inventoryv1.ReleaseInventoryRequest{OperationId: *op, OrderId: *order, Items: items(flags.Args())})
	case "query":
		res, err = client.QueryInventoryOperation(ctx, &inventoryv1.QueryInventoryOperationRequest{OperationId: *op})
	default:
		usage()
	}
	if err != nil {
		fail(err.Error())
	}
	fmt.Println(protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true}.Format(res))
}

func items(args []string) []*inventoryv1.InventoryItem {
	out := make([]*inventoryv1.InventoryItem, 0, len(args))
	for _, arg := range args {
		goods, quantity, ok := strings.Cut(arg, ":")
		g, err1 := strconv.ParseUint(goods, 10, 64)
		q, err2 := strconv.ParseUint(quantity, 10, 32)
		if !ok || err1 != nil || err2 != nil {
			fail("items are goods_id:quantity, got " + arg)
		}
		out = append(out, &inventoryv1.InventoryItem{GoodsId: g, Quantity: uint32(q)})
	}
	return out
}

func usage() {
	fail("usage: inventory reserve|commit|release|query [-order ID] [-op ID] [goods_id:quantity ...]")
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
