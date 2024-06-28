package bgp

import (
	"github.com/im-kulikov/docker-dns/internal/cacher"
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
	Attributes ConfigAttributes `env:"ATTRIBUTES"`
}

type ConfigAttributes struct {
	NextHop   string `env:"NEXT_HOP" default:"192.168.88.1"`
	LocalPref uint32 `env:"LOCAL_PREF" default:"100"`
}

//var lc net.ListenConfig
//lis, err := lc.Listen(ctx, "tcp", ":179")
//if err != nil {
//	panic(err)
//}
//
//routerID := netip.MustParseAddr("192.168.88.10")
//
//var srv *corebgp.Server
//if srv, err = corebgp.NewServer(routerID); err != nil {
//	panic(err)
//}
//
//p := &plugin{}
//err = srv.AddPeer(corebgp.PeerConfig{
//	RemoteAddress: netip.MustParseAddr("192.168.88.1"),
//	LocalAS:       65000,
//	RemoteAS:      65000,
//}, p, corebgp.WithLocalAddress(routerID))
//
//if err = srv.Serve([]net.Listener{lis}); err != nil {
//	panic(err)
//}

//list, err := net.InterfaceAddrs()
//if err != nil {
//	return nil, err
//}
//
//var ok bool
//for _, addr := range list {
//	var tmp *net.IPNet
//	if tmp, ok = addr.(*net.IPNet); !ok || tmp.IP.To4() == nil {
//		continue
//	}
//
//	local := netip.MustParseAddr(tmp.IP.String())
//	for _, client := range cfg.Clients {
//		err = srv.AddPeer(corebgp.PeerConfig{
//			RemoteAddress: netip.MustParseAddr(client),
//			LocalAS:       65000,
//			RemoteAS:      65000,
//		}, handler, corebgp.WithLocalAddress(local))
//	}
//}

type server struct {
	cfg Config
	log logger.Logger
	srv *corebgp.Server
}

// New creates a new BGP server.
func New(cfg Config, log logger.Logger, rec cacher.Interface) (Interface, error) {
	var err error
	handler := NewPlugin(log, cfg.Attributes, rec)
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

		if err = srv.AddPeer(conf, handler, corebgp.WithLocalAddress(rid)); err != nil {
			return nil, err
		}
	}

	return &server{cfg: cfg, log: log, srv: srv}, nil
}
