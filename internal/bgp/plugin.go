package bgp

import (
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	"go.uber.org/zap"
	"net/netip"

	"github.com/im-kulikov/go-bones/logger"
	"github.com/jwhited/corebgp"
)

type plugin struct {
	logger.Logger

	cfg ConfigAttributes
	rec cacher.Interface
}

func NewPlugin(log logger.Logger, cfg ConfigAttributes, rec cacher.Interface) corebgp.Plugin {
	return &plugin{Logger: log, cfg: cfg, rec: rec}
}

func (p *plugin) GetCapabilities(peer corebgp.PeerConfig) []corebgp.Capability {
	p.Infow("peer get capabilities", zap.Any("peer", peer))
	caps := make([]corebgp.Capability, 0)

	return caps
}

func (p *plugin) OnOpenMessage(peer corebgp.PeerConfig, routerID netip.Addr, capabilities []corebgp.Capability) *corebgp.Notification {
	p.Infow("peer open message", zap.Any("peer", peer))

	return nil
}

func (p *plugin) OnEstablished(peer corebgp.PeerConfig, writer corebgp.UpdateMessageWriter) corebgp.UpdateMessageHandler {
	p.Infow("peer established", zap.Any("peer", peer))

	// send peer updates
	if err := p.sendUpdateMessage(writer, p.rec); err != nil {
		return func(corebgp.PeerConfig, []byte) *corebgp.Notification {
			return corebgp.UpdateNotificationFromErr(err)
		}
	}

	// send End-of-Rib
	if err := writer.WriteUpdate([]byte{0, 0, 0, 0}); err != nil {
		return func(corebgp.PeerConfig, []byte) *corebgp.Notification {
			return corebgp.UpdateNotificationFromErr(err)
		}
	}

	return func(peer corebgp.PeerConfig, updateMessage []byte) *corebgp.Notification {
		var msg bgp.BGPUpdate
		if err := msg.DecodeFromBytes(updateMessage); err != nil {
			p.Errorw("could not decode update message", zap.Error(err))

			return corebgp.UpdateNotificationFromErr(err)
		}

		return nil
	}
}

func (p *plugin) OnClose(peer corebgp.PeerConfig) {
	p.Infow("peer closed", zap.Any("peer", peer))
}
