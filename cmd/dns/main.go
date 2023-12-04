package main

import (
	"context"
	"os/signal"
	"syscall"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/im-kulikov/go-bones/config"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"github.com/im-kulikov/go-bones/tracer"
	"github.com/im-kulikov/go-bones/web"

	dns "github.com/im-kulikov/docker-dns"
)

type settings struct {
	config.Base

	DNS dns.Config `env:"DNS"`
}

var (
	version = "dev"
	appName = "dns"
)

func (c settings) Validate(ctx context.Context) error {
	// check that your fields is ok...
	if err := c.DNS.Validate(ctx); err != nil {
		return err
	}

	return c.Base.Validate(ctx)
}

func main() {
	var cfg settings

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var err error
	if err = config.Load(ctx, &cfg); err != nil {
		logger.Default().Fatalf("could not prepare config: %s", err)
	}

	var log logger.Logger
	if log, err = logger.New(cfg.Base.Logger,
		logger.WithAppName(appName),
		logger.WithAppVersion(version)); err != nil {
		logger.Default().Fatalf("could not prepare logger: %s", err)
	}

	var trace service.Service
	if trace, err = tracer.Init(log, cfg.Base.Tracer); err != nil {
		log.Fatalf("could not initialize tracer: %s", err)
	}

	var cli *docker.Client
	if cli, err = docker.NewClientFromEnv(); err != nil {
		log.Fatalf("could not initialize docker client: %s", err)
	}

	svc := dns.NewServer(cfg.DNS, cli, log)
	ops := web.NewOpsServer(log, cfg.Base.Ops)

	var wrk dns.CacheWorker
	if wrk, err = dns.NewCache(cli, log); err != nil {
		log.Fatalf("could not initialize cache: %s", err)
	}

	svc.SetCache(wrk)

	group := service.New(log,
		service.WithService(svc),
		service.WithService(ops),
		service.WithService(wrk),
		service.WithService(trace),
		service.WithShutdownTimeout(cfg.Base.Shutdown))

	if err = group.Run(context.Background()); err != nil {
		log.Fatalf("something went wrong: %s", err)
	}
}
