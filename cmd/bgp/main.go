package main

import (
	"context"
	"github.com/im-kulikov/docker-dns/internal/bgp"
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/im-kulikov/docker-dns/internal/dns"
	"github.com/im-kulikov/go-bones/config"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"github.com/im-kulikov/go-bones/web"
	"go.uber.org/zap"
	"os"
	"os/signal"
)

type settings struct {
	config.Base

	BGP bgp.Config `env:"BGP"`
	DNS dns.Config `env:"DNS"`
}

func options(cfg settings, services ...service.Service) []service.Option {
	opts := make([]service.Option, 0, 1+len(services))

	opts = append(opts, service.WithShutdownTimeout(cfg.Shutdown))
	for _, svc := range services {
		opts = append(opts, service.WithService(svc))
	}

	return opts
}

func main() {
	var cfg settings

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var err error
	if err = config.Load(ctx, &cfg); err != nil {
		logger.Default().Panicw("could not load config", zap.Error(err))
	}

	var log logger.Logger
	if log, err = logger.New(cfg.Logger); err != nil {
		logger.Default().Panicw("could not create logger", zap.Error(err))
	}
	//
	//spew.Dump(cfg)
	//return

	var rec cacher.Interface
	if rec, err = cacher.New(); err != nil {
		log.Panicw("could not create cache", zap.Error(err))
	}

	var svc dns.Interface
	if svc, err = dns.New(cfg.DNS, log, rec); err != nil {
		log.Panicw("could not create dns service", zap.Error(err))
	}

	var srv bgp.Interface
	if srv, err = bgp.New(cfg.BGP, log, rec); err != nil {
		log.Panicw("could not create bgp service", zap.Error(err))
	}

	ops := web.NewOpsServer(log, cfg.Base.Ops)
	if err = service.New(log, options(cfg, svc, srv, ops)...).Run(ctx); err != nil {
		log.Panicw("could not create service runner", zap.Error(err))
	}
}
