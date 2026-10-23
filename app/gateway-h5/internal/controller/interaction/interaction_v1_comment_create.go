package interaction

import (
	"context"

	commentv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) CommentCreate(ctx context.Context, req *v1.CommentCreateReq) (res *v1.CommentCreateRes, err error) {
	grpcRes, err := c.CommentsClient.CreateComment(ctx, &commentv1.CreateCommentRequest{
		ObjectType: req.ObjectType,
		ObjectId:   req.ObjectId,
		ParentId:   req.ParentId,
		Content:    req.Content,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommentCreateRes{CommentId: grpcRes.CommentId}, nil
}
