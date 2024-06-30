package dns

import (
	"context"
	"sync"
	"time"

	"github.com/im-kulikov/docker-dns/internal/broadcast"
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"go.uber.org/zap"
)

type Config struct {
	Servers []string      `env:"SERVERS" default:""`
	Domains []string      `env:"DOMAINS" default:""`
	Enabled bool          `env:"ENABLED" default:"true"`
	Timeout time.Duration `env:"TIMEOUT" default:"5s"`
}

type Interface interface {
	service.Service
	cacher.Interface

	Enabled() bool
}

type server struct {
	sync.RWMutex

	cfg Config
	ext chan struct{}
	log logger.Logger
	rec cacher.Interface
	brd broadcast.Broadcaster

	cancel context.CancelFunc
}

func New(cfg Config, log logger.Logger, brd broadcast.Broadcaster) (Interface, error) {
	rec, err := cacher.New()
	if err != nil {
		return nil, err
	}

	return &server{
		cancel: func() {},

		cfg: cfg,
		log: log,
		brd: brd,
		rec: rec,
		ext: make(chan struct{}),
	}, nil
}

// Name returns the name of the server
func (*server) Name() string { return "dns-server" }

// Enabled returns true if the server is enabled
func (s *server) Enabled() bool { return s.cfg.Enabled }

type fetchResult struct {
	ttl time.Duration
	msg broadcast.UpdateMessage
}

func (s *server) fetchDomains(ctx context.Context, ttl time.Duration) fetchResult {
	ptx, cancel := context.WithTimeout(ctx, time.Second*15)
	defer cancel()

	s.RLock()
	out := make(chan fetchResult, len(s.cfg.Domains))
	for _, domain := range s.cfg.Domains {
		go s.resolve(ptx, out, domain)
	}

	received := len(s.cfg.Domains)
	s.RUnlock()

	var msg broadcast.UpdateMessage
loop:
	for {
		select {
		case <-ptx.Done():
			close(out)
			break loop

		case res, ok := <-out:
			if !ok {
				s.log.Infow("stop waiting for resolver response")

				break loop
			}

			msg.ToUpdate = append(msg.ToUpdate, res.msg.ToUpdate...)
			msg.ToRemove = append(msg.ToRemove, res.msg.ToRemove...)

			if res.ttl < ttl && res.ttl > 0 {
				ttl = res.ttl
			}

			received--
			if received <= 0 {

				s.log.Infow("received all answers")
				break loop
			}
		}
	}

	return fetchResult{
		ttl: ttl,
		msg: msg,
	}
}

// Start starts the DNS server
func (s *server) Start(ctx context.Context) error {
	ticker := time.NewTimer(time.Microsecond)
	defer ticker.Stop()

	ctx, s.cancel = context.WithCancel(ctx)
	for {
		select {
		case <-ctx.Done():
			s.log.Infow("dns resolver stopped")

			close(s.ext)
			s.cancel()

			return nil
		case <-ticker.C:
			now := time.Now()
			cur := s.fetchDomains(ctx, s.cfg.Timeout)

			s.log.Infow("resolved all domains",
				zap.Stringer("next", cur.ttl),
				zap.Stringer("spent", time.Since(now)))

			ticker.Reset(cur.ttl)
			s.brd.Broadcast(cur.msg)
		}
	}
}

// Stop stops the DNS server
func (s *server) Stop(ctx context.Context) {
	s.cancel()

	select {
	case <-ctx.Done():
		s.log.Infow("bgp context deadline for stop")
	case <-s.ext:
		s.log.Infow("bgp gracefully stopped")
	}
}
