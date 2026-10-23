// Package comment is the gRPC transport layer for comments.
package comment

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedCommentServiceServer
	comments service.IComment
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterCommentServiceServer(s.Server, &Controller{comments: logic.Comment()})
}

// ListComments is public: reading a product's comments needs no login.
func (c *Controller) ListComments(ctx context.Context, req *v1.ListCommentsRequest) (*v1.ListCommentsResponse, error) {
	items, total, err := c.comments.List(ctx, req.ObjectType, req.ObjectId, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	response := &v1.ListCommentsResponse{
		Items: make([]*v1.Comment, 0, len(items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: uint64(total),
	}
	for _, item := range items {
		response.Items = append(response.Items, &v1.Comment{
			CommentId:  item.ID,
			ParentId:   item.ParentID,
			UserId:     item.UserID,
			ObjectType: item.ObjectType,
			ObjectId:   item.ObjectID,
			Content:    item.Content,
			CreatedAt:  utility.SafeConvertTime(item.CreatedAt),
		})
	}
	return response, nil
}

func (c *Controller) CreateComment(ctx context.Context, req *v1.CreateCommentRequest) (*v1.CreateCommentResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := c.comments.Create(ctx, userID, &service.CommentInput{
		ObjectType: req.ObjectType,
		ObjectID:   req.ObjectId,
		ParentID:   req.ParentId,
		Content:    req.Content,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CreateCommentResponse{CommentId: id}, nil
}

func (c *Controller) DeleteComment(ctx context.Context, req *v1.DeleteCommentRequest) (*v1.DeleteCommentResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.comments.Delete(ctx, userID, req.CommentId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeleteCommentResponse{}, nil
}
