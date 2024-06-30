package broadcast

import (
	"context"

	"github.com/im-kulikov/go-bones/service"
	"github.com/jwhited/corebgp"
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

func (s *server) Stop(context.Context) {}
