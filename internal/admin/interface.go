package admin

import (
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
)

type Interface interface {
	service.Service

	Enabled() bool
}

type server struct {
	rec Storage
	log logger.Logger
}
