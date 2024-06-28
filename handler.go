package dns

import (
	"strings"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

type question []dns.Question

func (q question) String() string {
	var s []string
	for _, item := range q {
		s = append(s, item.String())
	}

	return strings.Join(s, ", ")
}

func (s *server) externalExchange(req, out *dns.Msg) error {
	if len(out.Answer) > 0 {
		return ErrAlreadySet
	}

	s.Once.Do(func() { s.client = &dns.Client{Net: "udp"} })

	s.logger.Debugw("exchange with Google DNS")

	res, _, err := s.client.Exchange(req, "8.8.8.8:53")
	if err != nil {
		return err
	}

	res.CopyTo(out)

	if len(out.Answer) > 0 {
		return ErrBreak
	}

	return nil
}

func (s *server) internalExchange(req, out *dns.Msg) error {
	if len(out.Answer) > 0 {
		return ErrAlreadySet
	}

	s.logger.Debugw("exchange with Docker DNS")

	for _, q := range req.Question {
		s.logger.Debugw("resolving dns",
			Query(q).Fields()...)

		rec, err := s.stores.Get(q)
		if err != nil {
			s.logger.Warnw("fetch record failed",
				Query(q).Fields(zap.Error(err))...)

			continue
		}

		out.Answer = append(out.Answer, rec...)
	}

	if len(out.Answer) > 0 {
		return ErrBreak
	}

	return nil
}

func (s *server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	reply := &dns.Msg{}
	reply.SetReply(req)

	resolvers := []func(req, reply *dns.Msg) error{
		s.internalExchange,
		s.externalExchange}

	for _, resolver := range resolvers {
		switch err := resolver(req, reply); err {
		default:
			s.logger.Errorw("could not exchange with Docker DNS",
				Queries(req.Question).Fields(zap.Error(err))...)

			continue
		case ErrAlreadySet, ErrBreak:
			break
		case nil:
			continue
		}
	}

	err := w.WriteMsg(reply)
	if err != nil {
		s.logger.Errorw("could not write reply",
			Queries(req.Question).Fields(zap.Error(err))...)
	}
}
