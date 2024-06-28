package bgp

import (
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/jwhited/corebgp"
	"github.com/osrg/gobgp/pkg/packet/bgp"
	"go.uber.org/zap"
)

func (p *plugin) sendUpdateMessage(writer corebgp.UpdateMessageWriter, peers cacher.Interface) error {
	var updated, removed int

	var err error
	peers.Range(func(domain string, item *cacher.CacheItem) bool {
		update := item.UpdateMessage()

		updated += len(update.ToUpdate)
		removed += len(update.ToRemove)

		removes := make([]*bgp.IPAddrPrefix, 0, len(update.ToRemove))
		for _, address := range update.ToRemove {
			removes = append(removes, bgp.NewIPAddrPrefix(32, address))
		}

		updates := make([]*bgp.IPAddrPrefix, 0, len(update.ToUpdate))
		for _, address := range update.ToUpdate {
			updates = append(updates, bgp.NewIPAddrPrefix(32, address))
		}

		attributes := make([]bgp.PathAttributeInterface, 0, 4)
		attributes = append(attributes,
			bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP),
			bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{}),
			bgp.NewPathAttributeNextHop(p.cfg.NextHop),
			bgp.NewPathAttributeLocalPref(p.cfg.LocalPref))

		out := &bgp.BGPUpdate{
			WithdrawnRoutesLen:    uint16(len(removes)),
			WithdrawnRoutes:       removes,
			TotalPathAttributeLen: uint16(len(attributes)),
			PathAttributes:        attributes,
			NLRI:                  updates,
		}

		var msg []byte
		if msg, err = out.Serialize(); err != nil {
			return false
		} else if err = writer.WriteUpdate(msg); err != nil {
			return false
		}

		return true
	})

	p.Infow("update sent",
		zap.Int("updated", updated),
		zap.Int("removed", removed),
		zap.Error(err))

	return err
}
