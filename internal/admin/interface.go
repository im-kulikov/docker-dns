package admin

import (
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
)

type Interface interface {
	service.Service

	Enabled() bool
}

type server struct {
	log logger.Logger
	rec cacher.Interface
}
