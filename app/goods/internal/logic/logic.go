// Package logic wires the application services to their dependencies.
//
// Init must run once at startup, before any controller is registered, because
// the cache and the file service client are shared by both services.
package logic

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/cache"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/filelookup"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic/cart"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic/category"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic/goods"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic/inventory"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

var (
	goodsService     service.IGoods
	categoryService  service.ICategory
	inventoryService service.IInventory
	cartService      service.ICart
)

// Init builds the services. It fails only on a malformed cache config; an
// unreachable Redis is tolerated, see cache.FromConfig.
func Init(ctx context.Context) error {
	c, err := cache.FromConfig(ctx)
	if err != nil {
		return err
	}
	files := filelookup.New()
	goodsRepo, categoryRepo := repository.NewGoods(), repository.NewCategory()
	stocks, images := repository.NewStock(), repository.NewImage()
	goodsService = goods.New(goodsRepo, stocks, images, categoryRepo, files, c)
	categoryService = category.New(categoryRepo, goodsRepo, files, c)
	inventoryService = inventory.New(repository.NewLedger())
	cartService = cart.New(repository.NewCart(), goodsRepo, images, stocks)
	return nil
}

func Goods() service.IGoods         { return goodsService }
func Category() service.ICategory   { return categoryService }
func Inventory() service.IInventory { return inventoryService }
func Cart() service.ICart           { return cartService }
