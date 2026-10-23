package interaction

import (
	"context"

	commentv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

func (c *ControllerV1) CommentList(ctx context.Context, req *v1.CommentListReq) (res *v1.CommentListRes, err error) {
	grpcRes, err := c.CommentsClient.ListComments(ctx, &commentv1.ListCommentsRequest{
		ObjectType: req.ObjectType,
		ObjectId:   req.ObjectId,
		Page:       req.Page,
		Size:       req.Size,
	})
	if err != nil {
		return nil, err
	}

	res = &v1.CommentListRes{
		Page:  req.Page,
		Size:  req.Size,
		Total: grpcRes.Total,
		Items: make([]*v1.CommentItem, 0, len(grpcRes.Items)),
	}
	for _, item := range grpcRes.Items {
		res.Items = append(res.Items, &v1.CommentItem{
			CommentId:  item.CommentId,
			ParentId:   item.ParentId,
			UserId:     item.UserId,
			ObjectType: item.ObjectType,
			ObjectId:   item.ObjectId,
			Content:    item.Content,
			CreatedAt:  utility.TimeFromTimestamp(item.CreatedAt),
		})
	}
	return res, nil
}
