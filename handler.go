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
	s.Once.Do(func() { s.client = &dns.Client{Net: "udp"} })

	//for _, question := range req.Question {
	//	if question.Name == "." {
	//		return fmt.Errorf("ignore, root query not supported")
	//	}
	//}

	s.logger.Info("exchange with Google DNS")

	res, _, err := s.client.Exchange(req, "8.8.8.8:53")
	if err != nil {
		return err
	}

	res.CopyTo(out)

	return nil
}

func (s *server) internalExchange(req, out *dns.Msg) error {
	s.logger.Info("exchange with Docker DNS")

	for _, q := range req.Question {
		s.logger.Infow("resolving dns",
			zap.Uint16("type", q.Qtype),
			zap.String("name", q.Name))

		rec, err := s.stores.Get(q)
		if err != nil {
			s.logger.Warnw("fetch record failed",
				zap.Uint16("type", q.Qtype),
				zap.String("name", q.Name),
				zap.Error(err))

			continue
		}

		out.Answer = append(out.Answer, rec...)
	}

	return nil
}

func (s *server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	reply := &dns.Msg{}
	reply.SetReply(req)

	// handle external queries
	if err := s.externalExchange(req, reply); err != nil {
		s.logger.Warnw("could not exchange with Google DNS",
			zap.String("query", question(req.Question).String()),
			zap.Error(err))
	}

	// handle internal queries
	if err := s.internalExchange(req, reply); err != nil {
		s.logger.Error("could not exchange with Docker DNS",
			zap.String("query", question(req.Question).String()),
			zap.Error(err))
	}

	err := w.WriteMsg(reply)
	if err != nil {
		s.logger.Error("could not write reply",
			zap.String("query", question(req.Question).String()),
			zap.Error(err))
	}
}
