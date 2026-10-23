// Package logic wires the application service to its repository.
package logic

import (
	"github.com/hughtaiyaki/goframeshop/app/file/internal/logic/file"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/service"
)

func File() service.IFile {
	return file.New(repository.NewFile())
}
