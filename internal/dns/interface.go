package dns

import (
	"context"
	"github.com/im-kulikov/docker-dns/internal/broadcast"
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"go.uber.org/zap"
	"time"
)

type Config struct {
	Servers []string      `env:"SERVERS" default:""`
	Domains []string      `env:"DOMAINS" default:""`
	Enabled bool          `env:"ENABLED" default:"true"`
	Timeout time.Duration `env:"TIMEOUT" default:"5s"`
}

type Interface interface {
	service.Service
}

type server struct {
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

func (s *server) fetchDomains(ctx context.Context, ttl time.Duration, out chan time.Duration) time.Duration {
	ptx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	for _, domain := range s.cfg.Domains {
		go s.resolve(ptx, out, domain)
	}

	received := len(s.cfg.Domains)
loop:
	for {
		select {
		case <-ptx.Done():
			break loop

		case current, ok := <-out:
			if !ok {
				s.log.Infow("stop waiting for resolver response")

				break loop
			}

			if current < ttl && current > 0 {
				ttl = current
			}

			received--
			if received <= 0 {

				s.log.Infow("received all answers")
				break loop
			}
		}
	}

	return ttl
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

			ttl := s.cfg.Timeout
			out := make(chan time.Duration, len(s.cfg.Domains))
			if tmp := s.fetchDomains(ctx, ttl, out); tmp < ttl && tmp > 0 {
				ttl = tmp
			}

			now := time.Now()

			s.log.Infow("resolved all domains",
				zap.Duration("next", ttl),
				zap.Duration("spent", time.Since(now)))

			ticker.Reset(ttl)
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
