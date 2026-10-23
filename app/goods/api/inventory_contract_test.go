package api_test

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"

	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
)

// Regression: the items rule was "length:1,50", which GoFrame applies to the
// JSON text of the list, so every order with two or more lines was refused
// as InvalidArgument. The count limit lives in the service instead.
func TestInventoryRequestsAcceptManyItems(t *testing.T) {
	items := make([]*inventoryv1.InventoryItem, 0, 50)
	for i := uint64(1); i <= 50; i++ {
		items = append(items, &inventoryv1.InventoryItem{GoodsId: i, Quantity: 99})
	}
	for name, req := range map[string]any{
		"reserve": &inventoryv1.ReserveInventoryRequest{OperationId: "reserve-o1", OrderId: "o1", Items: items},
		"release": &inventoryv1.ReleaseInventoryRequest{OperationId: "release-o1", OrderId: "o1", Items: items},
	} {
		if err := g.Validator().Data(req).Run(context.Background()); err != nil {
			t.Errorf("%s with 50 items: %v", name, err)
		}
	}
	empty := &inventoryv1.ReserveInventoryRequest{OperationId: "reserve-o1", OrderId: "o1"}
	if err := g.Validator().Data(empty).Run(context.Background()); err == nil {
		t.Error("reserve with no items passed validation")
	}
}
