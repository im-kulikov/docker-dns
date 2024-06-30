package dns

import (
	"context"
	"time"

	"github.com/im-kulikov/docker-dns/internal/broadcast"
	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func (s *server) resolve(ctx context.Context, out chan fetchResult, domain string) {
	rec, ok := s.rec.Get(domain)
	if !ok {
		rec = cacher.NewItem(domain)
	} else if rec.IsExpired() {
		s.log.Debugw("cache expired",
			zap.String("domain", domain),
			zap.Uint32("ttl", rec.Expire),
			zap.Strings("records", rec.Record))

		rec.Reset()
	} else {
		s.log.Debugw("cache hit",
			zap.String("domain", domain),
			zap.Strings("records", rec.Record))

		return
	}

	query := dns.Question{Name: domain + ".", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	s.log.Debugw("trying to resolve",
		zap.String("domain", domain),
		zap.Uint32("ttl", rec.Expire))

	var res broadcast.UpdateMessage
	msg := &dns.Msg{
		MsgHdr:   dns.MsgHdr{RecursionDesired: true},
		Question: []dns.Question{query},
	}

	var err error
	for _, srv := range s.cfg.Servers {
		var result *dns.Msg
		if result, err = dns.ExchangeContext(ctx, msg, srv); err != nil {
			s.log.Debugw("could not resolve",
				zap.String("domain", domain),
				zap.String("server", srv),
				zap.Error(err))

			return
		}

		ttl := rec.Expire
		var tmp []string
		for _, rr := range result.Answer {
			switch r := rr.(type) {
			case *dns.A:
				ttl = r.Hdr.Ttl
				tmp = append(tmp, r.A.String())
			}
		}

		upd := rec.AddRecords(tmp, ttl)
		res.ToUpdate = append(res.ToUpdate, upd.ToUpdate...)
		res.ToRemove = append(res.ToRemove, upd.ToRemove...)
	}

	s.rec.Set(domain, rec)
	s.log.Debugw("resolved",
		zap.String("domain", domain),
		zap.Uint32("ttl", rec.Expire),
		zap.Strings("records", rec.Record))

	out <- fetchResult{msg: res, ttl: time.Second * time.Duration(rec.Expire)}
}
