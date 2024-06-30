package admin

import (
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"github.com/im-kulikov/go-bones/web"
)

func New(cfg web.HTTPConfig, log logger.Logger, rec cacher.Interface) service.Service {
	srv := &server{rec: rec, log: log}

	return web.NewHTTPServer(
		web.WithHTTPConfig(cfg),
		web.WithHTTPLogger(log),
		web.WithHTTPHandler(srv.router()),
		web.WithHTTPName("admin-server"))
}
