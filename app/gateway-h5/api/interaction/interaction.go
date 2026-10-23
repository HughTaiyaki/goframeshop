// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package interaction

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/interaction/v1"
)

type IInteractionV1 interface {
	CollectionCreate(ctx context.Context, req *v1.CollectionCreateReq) (res *v1.CollectionCreateRes, err error)
	CollectionDelete(ctx context.Context, req *v1.CollectionDeleteReq) (res *v1.CollectionDeleteRes, err error)
	CollectionList(ctx context.Context, req *v1.CollectionListReq) (res *v1.CollectionListRes, err error)
	CommentList(ctx context.Context, req *v1.CommentListReq) (res *v1.CommentListRes, err error)
	CommentCreate(ctx context.Context, req *v1.CommentCreateReq) (res *v1.CommentCreateRes, err error)
	CommentDelete(ctx context.Context, req *v1.CommentDeleteReq) (res *v1.CommentDeleteRes, err error)
	PraiseCreate(ctx context.Context, req *v1.PraiseCreateReq) (res *v1.PraiseCreateRes, err error)
	PraiseDelete(ctx context.Context, req *v1.PraiseDeleteReq) (res *v1.PraiseDeleteRes, err error)
	PraiseList(ctx context.Context, req *v1.PraiseListReq) (res *v1.PraiseListRes, err error)
}
