package broadcast

import (
	"context"
	"github.com/containerd/containerd/pkg/atomic"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	"go.uber.org/zap"
	"sync"
	"time"

	"github.com/im-kulikov/go-bones/logger"
	"github.com/jwhited/corebgp"
)

type server struct {
	sync.RWMutex
	logger.Logger

	cfg Config
	ext atomic.Bool
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
		ext: atomic.NewBool(false),
		act: make(chan updatePeer, 10),
		out: make(chan UpdateMessage, 10),
	}
}

func (s *server) DelPeer(peer string) {
	if s.ext.IsSet() {
		return
	}

	s.act <- updatePeer{Peer: peer, Action: remPeer}
}

func (s *server) AddPeer(peer string, writer corebgp.UpdateMessageWriter) {
	if s.ext.IsSet() {
		return
	}

	s.act <- updatePeer{Peer: peer, Action: addPeer, writer: writer}
}

func (s *server) Broadcast(msg UpdateMessage) {
	if s.ext.IsSet() {
		return
	}

	s.out <- msg
}

func (s *server) updateList(list []string, msg UpdateMessage) []string {
	if len(msg.ToUpdate) == 0 && len(msg.ToRemove) == 0 {
		return list
	}

	s.Infow("before update",
		zap.Int("msg.update", len(msg.ToUpdate)),
		zap.Int("msg.remove", len(msg.ToRemove)),
		zap.Int("list", len(list)))

	// Создаем карту для отслеживания элементов списка
	listMap := make(map[string]bool)
	for _, item := range list {
		listMap[item] = true
	}

	// Удаляем элементы, которые должны быть удалены
	for _, item := range msg.ToRemove {
		delete(listMap, item)
	}

	// Добавляем новые элементы
	for _, item := range msg.ToUpdate {
		listMap[item] = true
	}

	// Создаем обновленный список из карты
	updatedList := make([]string, 0, len(listMap))
	for item := range listMap {
		updatedList = append(updatedList, item)
	}

	s.Infow("after update",
		zap.Int("msg.update", len(msg.ToUpdate)),
		zap.Int("msg.remove", len(msg.ToRemove)),
		zap.Int("list", len(updatedList)))

	return updatedList
}

func (s *server) sendInitialTables(writer corebgp.UpdateMessageWriter, msg UpdateMessage) error {
	if len(msg.ToUpdate) == 0 && len(msg.ToRemove) == 0 {
		return nil
	}

	removes := make([]*bgp.IPAddrPrefix, 0, len(msg.ToRemove))
	for _, address := range msg.ToRemove {
		removes = append(removes, bgp.NewIPAddrPrefix(32, address))
	}

	attributes := make([]bgp.PathAttributeInterface, 0, 4)
	attributes = append(attributes,
		bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
		bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{}),
		bgp.NewPathAttributeNextHop(s.cfg.NextHop),
		bgp.NewPathAttributeLocalPref(s.cfg.LocalPref))

	// send batches of 1000 updates
	for i := 0; i < len(msg.ToUpdate); i += 1000 {
		end := i + 1000
		if end > len(msg.ToUpdate) {
			end = len(msg.ToUpdate)
		}

		updates := make([]*bgp.IPAddrPrefix, 0, len(msg.ToUpdate[i:end]))
		for _, address := range msg.ToUpdate[i:end] {
			updates = append(updates, bgp.NewIPAddrPrefix(32, address))
		}

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
		} else if err = writer.WriteUpdate([]byte{0, 0, 0, 0}); err != nil {
			return err
		}

		removes = nil // remove withdrawn routes after the first batch
	}

	return writer.WriteUpdate([]byte{0, 0, 0, 0})
}

func (s *server) Start(ctx context.Context) error {
	var (
		list []string
		peer = make(map[string]corebgp.UpdateMessageWriter)
	)

	ticker := time.NewTimer(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Infow("broadcaster stopped")

			s.ext.Set()
			close(s.act)
			close(s.out)

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
				s.Infow("remove peer writer", zap.String("peer", msg.Peer))
				delete(peer, msg.Peer)
			default:
				s.Infow("unknown Action", zap.Any("Action", msg))
			}
		case msg := <-s.out:
			if len(msg.ToUpdate) == 0 && len(msg.ToRemove) == 0 {
				s.Debugw("ignore empty update")

				continue
			}

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
