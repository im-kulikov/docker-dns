package bgp

import (
	"context"
	"github.com/im-kulikov/go-bones/service"
	"go.uber.org/zap"
	"net"
)

// Interface is a service interface for BGP server
type Interface service.Service

// Name implements service.Service interface
func (*server) Name() string { return "bgp-server" }

// Enabled returns true if the server is enabled
func (s *server) Enabled() bool { return s.cfg.Enabled }

// Start implements service.Service interface
func (s *server) Start(ctx context.Context) error {
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, s.cfg.Network, s.cfg.Address)
	if err != nil {
		return err
	}

	s.log.Infow("listening", zap.String("address", s.cfg.Address))

	defer func() { s.log.Infow("bgp server stopped") }()

	return s.srv.Serve([]net.Listener{lis})
}

// Stop implements service.Service interface
func (s *server) Stop(context.Context) {
	s.srv.Close()
}
