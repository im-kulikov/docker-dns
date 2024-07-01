package bgp

import (
	"net/netip"
	"time"

	"github.com/im-kulikov/docker-dns/internal/broadcast"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/jwhited/corebgp"
	"go.uber.org/zap"
)

type plugin struct {
	logger.Logger

	rec broadcast.PeerManager
}

func newPlugin(log logger.Logger, rec broadcast.PeerManager) corebgp.Plugin {
	return &plugin{Logger: log, rec: rec}
}

func (p *plugin) GetCapabilities(peer corebgp.PeerConfig) []corebgp.Capability {
	p.Infow("peer get capabilities", zap.Any("peer", peer))
	caps := make([]corebgp.Capability, 0)

	return caps
}

func (p *plugin) OnOpenMessage(peer corebgp.PeerConfig, _ netip.Addr, _ []corebgp.Capability) *corebgp.Notification {
	p.Infow("peer open message", zap.Any("peer", peer))

	return nil
}

func (p *plugin) OnEstablished(peer corebgp.PeerConfig, writer corebgp.UpdateMessageWriter) corebgp.UpdateMessageHandler {
	p.Infow("peer established", zap.Any("peer", peer))
	p.rec.AddPeer(peer.RemoteAddress.String(), writer)

	time.Sleep(time.Second) // wait before send initial update

	// send End-of-Rib
	if err := writer.WriteUpdate([]byte{0, 0, 0, 0}); err != nil {
		return func(corebgp.PeerConfig, []byte) *corebgp.Notification {
			return corebgp.UpdateNotificationFromErr(err)
		}
	}

	return nil // ignore client updates
}

func (p *plugin) OnClose(peer corebgp.PeerConfig) {
	p.Infow("peer closed", zap.Any("peer", peer))

	p.rec.DelPeer(peer.RemoteAddress.String())
}
