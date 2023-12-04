package dns

import (
	"context"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

type server struct {
	sync.Once

	stores Cacher
	server *dns.Server
	client *dns.Client
	logger logger.Logger
}

type Server interface {
	service.Service

	SetCache(Cacher)
}

func NewServer(cfg Config, cli *docker.Client, log logger.Logger) Server {
	return &server{
		logger: log,
		stores: &dockerStore{
			client: cli,
			logger: log,
		},
		server: &dns.Server{
			Net:  cfg.Network,
			Addr: cfg.Address,
		},
	}
}

func (s *server) Name() string { return "docker-dns" }

func (s *server) SetCache(v Cacher) {
	if store, ok := s.stores.(*dockerStore); ok {
		store.cacher = v
	}

	s.stores = &chainStore{stores: []Cacher{v, s.stores}}
}

func (s *server) Start(_ context.Context) error {
	s.server.Handler = s

	return s.server.ListenAndServe()
}

func (s *server) Stop(ctx context.Context) {
	if err := s.server.ShutdownContext(ctx); err != nil {
		s.logger.Errorw("could not shutdown server", zap.Error(err))
	}
}
