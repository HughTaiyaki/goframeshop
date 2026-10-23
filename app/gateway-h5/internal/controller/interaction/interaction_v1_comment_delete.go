package interaction

import (
	"context"

	commentv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) CommentDelete(ctx context.Context, req *v1.CommentDeleteReq) (res *v1.CommentDeleteRes, err error) {
	if _, err := c.CommentsClient.DeleteComment(ctx, &commentv1.DeleteCommentRequest{CommentId: req.CommentId}); err != nil {
		return nil, err
	}
	return &v1.CommentDeleteRes{}, nil
}
