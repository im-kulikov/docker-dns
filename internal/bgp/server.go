package bgp

import (
	"github.com/im-kulikov/docker-dns/internal/broadcast"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/jwhited/corebgp"
	"go.uber.org/zap"
	"net/netip"
)

type Config struct {
	Clients    []string         `env:"CLIENTS" default:""`
	Enabled    bool             `env:"ENABLED" default:"true"`
	Network    string           `env:"NETWORK" default:"tcp"`
	Address    string           `env:"ADDRESS" default:":51179"`
	RouteID    string           `env:"ROUTER_ID" default:"127.0.0.1"`
	Attributes broadcast.Config `env:"ATTRIBUTES"`
}

type server struct {
	cfg Config
	log logger.Logger
	srv *corebgp.Server
}

// New creates a new BGP server.
func New(cfg Config, log logger.Logger, rec broadcast.PeerManager) (Interface, error) {
	var err error
	handler := newPlugin(log, rec)
	log.Infow("bgp server", zap.Any("config", cfg))

	var rid netip.Addr
	if rid, err = netip.ParseAddr(cfg.RouteID); err != nil {
		return nil, err
	}

	var srv *corebgp.Server
	if srv, err = corebgp.NewServer(rid); err != nil {
		return nil, err
	}

	for _, client := range cfg.Clients {
		conf := corebgp.PeerConfig{
			RemoteAddress: netip.MustParseAddr(client),
			LocalAS:       65000,
			RemoteAS:      65000,
		}

		log.Infow("adding peer", zap.Any("peer", conf), zap.String("router_id", cfg.RouteID))

		if err = srv.AddPeer(conf, handler, corebgp.WithLocalAddress(rid), corebgp.WithPassive()); err != nil {
			return nil, err
		}
	}

	return &server{cfg: cfg, log: log, srv: srv}, nil
}
