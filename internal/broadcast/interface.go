package broadcast

import (
	"context"
	"github.com/im-kulikov/go-bones/service"
	"github.com/jwhited/corebgp"
	"time"
)

type Interface interface {
	Broadcaster
	PeerManager
	service.Service
}

type PeerManager interface {
	DelPeer(string)
	AddPeer(string, corebgp.UpdateMessageWriter)
}

type Broadcaster interface {
	Broadcast(msg UpdateMessage)
}

func (*server) Name() string { return "broadcaster" }

func (s *server) Stop(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	select {
	case <-ctx.Done():
	case <-s.ext:
	}
}
