// Package logic wires the application service to its repository.
package logic

import (
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/logic/admin"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/service"
)

func Admin() service.IAdmin {
	return admin.New(repository.NewAdmin())
}
