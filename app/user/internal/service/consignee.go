package service

import (
	"context"
)

// ConsigneeInput holds the writable fields of an address. UserID is never taken
// from the request: the caller identity decides which user an address belongs to.
type ConsigneeInput struct {
	IsDefault bool
	Name      string
	Phone     string
	Province  string
	City      string
	Town      string
	Street    string
	Detail    string
}

// IConsignee is the shipping address application service.
//
// Every method takes the authenticated userID as an explicit argument so that a
// missing identity is a compile-time concern rather than a runtime surprise.
type IConsignee interface {
	List(ctx context.Context, userID uint64, page, size int) ([]*Consignee, int, error)
	Create(ctx context.Context, userID uint64, in *ConsigneeInput) (uint64, error)
	Update(ctx context.Context, userID, consigneeID uint64, in *ConsigneeInput) error
	Delete(ctx context.Context, userID, consigneeID uint64) error
	Get(ctx context.Context, userID, consigneeID uint64) (*Consignee, error)
}
