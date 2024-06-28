package broadcast

import (
	"context"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	"go.uber.org/zap"
	"sync"

	"github.com/im-kulikov/go-bones/logger"
	"github.com/jwhited/corebgp"
)

type server struct {
	sync.RWMutex
	logger.Logger

	cfg Config
	ext chan struct{}
	act chan updatePeer
	out chan UpdateMessage
}

// UpdateMessage represents a message to update the DNS records
type UpdateMessage struct {
	ToUpdate []string
	ToRemove []string
}

type updatePeer struct {
	Peer   string
	Action action
	writer corebgp.UpdateMessageWriter
}

type Config struct {
	NextHop   string `env:"NEXT_HOP" default:"192.168.88.1"`
	LocalPref uint32 `env:"LOCAL_PREF" default:"100"`
}

type action uint8

const (
	_ action = iota
	addPeer
	remPeer
)

func New(cfg Config, log logger.Logger) Interface {
	return &server{
		Logger: log,

		cfg: cfg,
		ext: make(chan struct{}),
		act: make(chan updatePeer, 10),
		out: make(chan UpdateMessage, 10),
	}
}

func (s *server) DelPeer(peer string) {
	select {
	case <-s.ext:
		s.Warnw("could not send remove peer", zap.String("peer", peer))
	case s.act <- updatePeer{Peer: peer, Action: remPeer}:
	}
}

func (s *server) AddPeer(peer string, writer corebgp.UpdateMessageWriter) {
	select {
	case <-s.ext:
		s.Warnw("could not send add peer", zap.String("peer", peer))
	case s.act <- updatePeer{Peer: peer, Action: addPeer, writer: writer}:
	}
}

func (s *server) Broadcast(msg UpdateMessage) { s.out <- msg }

func (s *server) updateList(list []string, msg UpdateMessage) []string {
	out := make([]string, 0, len(list))
	for _, cur := range list {
		for _, rem := range msg.ToRemove {
			if cur == rem {
				continue
			}

			out = append(out, cur)
		}
	}

	for _, add := range msg.ToUpdate {
		out = append(out, add)
	}

	return out
}

func (s *server) sendInitialTables(writer corebgp.UpdateMessageWriter, msg UpdateMessage) error {
	removes := make([]*bgp.IPAddrPrefix, 0, len(msg.ToRemove))
	for _, address := range msg.ToRemove {
		removes = append(removes, bgp.NewIPAddrPrefix(32, address))
	}

	updates := make([]*bgp.IPAddrPrefix, 0, len(msg.ToUpdate))
	for _, address := range msg.ToUpdate {
		updates = append(updates, bgp.NewIPAddrPrefix(32, address))
	}

	attributes := make([]bgp.PathAttributeInterface, 0, 4)
	attributes = append(attributes,
		bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
		bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{}),
		bgp.NewPathAttributeNextHop(s.cfg.NextHop),
		bgp.NewPathAttributeLocalPref(s.cfg.LocalPref))

	out := &bgp.BGPUpdate{
		WithdrawnRoutesLen:    uint16(len(removes)),
		WithdrawnRoutes:       removes,
		TotalPathAttributeLen: uint16(len(attributes)),
		PathAttributes:        attributes,
		NLRI:                  updates,
	}

	if buf, err := out.Serialize(); err != nil {
		return err
	} else if err = writer.WriteUpdate(buf); err != nil {
		return err
	}

	return writer.WriteUpdate([]byte{0, 0, 0, 0})
}

func (s *server) Start(ctx context.Context) error {
	var (
		list []string
		peer = make(map[string]corebgp.UpdateMessageWriter)
	)

	for {
		select {
		case <-ctx.Done():
			s.Infow("broadcaster stopped")

			close(s.act)
			close(s.out)
			close(s.ext)

			return nil
		case msg := <-s.act:
			switch msg.Action {
			case addPeer:
				peer[msg.Peer] = msg.writer

				err := s.sendInitialTables(msg.writer, UpdateMessage{ToUpdate: list})
				s.Infow("send initial table",
					zap.String("peer", msg.Peer),
					zap.Int("updates", len(list)),
					zap.Error(err))

			case remPeer:
				delete(peer, msg.Peer)
			default:
				s.Infow("unknown Action", zap.Any("Action", msg))
			}
		case msg := <-s.out:
			list = s.updateList(list, msg)

			for client, writer := range peer {
				err := s.sendInitialTables(writer, msg)
				s.Infow("send update message",
					zap.String("peer", client),
					zap.Int("updates", len(msg.ToUpdate)),
					zap.Int("removes", len(msg.ToRemove)),
					zap.Error(err))
			}
		}
	}
}
