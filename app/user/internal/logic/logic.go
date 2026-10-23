// Package logic wires the application services to their repositories.
//
// Controllers call logic.User() / logic.Consignee() instead of constructing
// services themselves, so there is exactly one place that knows which
// repository implementation is in use.
package logic

import (
	"github.com/hughtaiyaki/goframeshop/app/user/internal/logic/consignee"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/logic/user"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

func User() service.IUser {
	return user.New(repository.NewUser())
}

func Consignee() service.IConsignee {
	return consignee.New(repository.NewConsignee())
}
