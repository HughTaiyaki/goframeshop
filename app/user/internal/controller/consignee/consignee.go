// Package consignee is the gRPC transport layer for the address API.
package consignee

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedConsigneeServiceServer
	consignees service.IConsignee
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterConsigneeServiceServer(s.Server, &Controller{consignees: logic.Consignee()})
}

func (c *Controller) ListConsignees(ctx context.Context, req *v1.ListConsigneesRequest) (*v1.ListConsigneesResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := c.consignees.List(ctx, userID, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	response := &v1.ListConsigneesResponse{
		Items: make([]*v1.Consignee, 0, len(items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: uint64(total),
	}
	for _, item := range items {
		response.Items = append(response.Items, toProto(item))
	}
	return response, nil
}

func (c *Controller) CreateConsignee(ctx context.Context, req *v1.CreateConsigneeRequest) (*v1.CreateConsigneeResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := c.consignees.Create(ctx, userID, &service.ConsigneeInput{
		IsDefault: req.IsDefault,
		Name:      req.Name,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		Town:      req.Town,
		Street:    req.Street,
		Detail:    req.Detail,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CreateConsigneeResponse{ConsigneeId: id}, nil
}

func (c *Controller) UpdateConsignee(ctx context.Context, req *v1.UpdateConsigneeRequest) (*v1.UpdateConsigneeResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	err = c.consignees.Update(ctx, userID, req.ConsigneeId, &service.ConsigneeInput{
		IsDefault: req.IsDefault,
		Name:      req.Name,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		Town:      req.Town,
		Street:    req.Street,
		Detail:    req.Detail,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.UpdateConsigneeResponse{ConsigneeId: req.ConsigneeId}, nil
}

func (c *Controller) DeleteConsignee(ctx context.Context, req *v1.DeleteConsigneeRequest) (*v1.DeleteConsigneeResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.consignees.Delete(ctx, userID, req.ConsigneeId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeleteConsigneeResponse{}, nil
}

// requireUserID reads the identity that the auth interceptor verified.
func requireUserID(ctx context.Context) (uint64, error) {
	userID, ok := utility.UserIDFromContext(ctx)
	if !ok {
		return 0, controller.StatusError(service.ErrNoIdentity)
	}
	return userID, nil
}

// toProto converts a domain address into the public message. The public message
// is intentionally not pbentity.ConsigneeInfo: the database shape and the API
// shape must be free to evolve separately.
func toProto(item *service.Consignee) *v1.Consignee {
	return &v1.Consignee{
		ConsigneeId: item.ID,
		IsDefault:   item.IsDefault,
		Name:        item.Name,
		Phone:       item.Phone,
		Province:    item.Province,
		City:        item.City,
		Town:        item.Town,
		Street:      item.Street,
		Detail:      item.Detail,
		CreatedAt:   utility.SafeConvertTime(item.CreatedAt),
		UpdatedAt:   utility.SafeConvertTime(item.UpdatedAt),
	}
}

func (c *Controller) GetConsignee(ctx context.Context, req *v1.GetConsigneeRequest) (*v1.GetConsigneeResponse, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.consignees.Get(ctx, userID, req.ConsigneeId)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.GetConsigneeResponse{Consignee: toProto(item)}, nil
}
