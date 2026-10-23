// Package logic wires the application services to their repositories.
//
// Controllers call logic.Praise() / logic.Collection() / logic.Comment()
// instead of constructing services themselves, so there is exactly one place
// that knows which repository implementation is in use.
package logic

import (
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/logic/comment"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/logic/mark"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
)

func Praise() service.IMark {
	return mark.New(repository.NewPraise())
}

func Collection() service.IMark {
	return mark.New(repository.NewCollection())
}

func Comment() service.IComment {
	return comment.New(repository.NewComment())
}
