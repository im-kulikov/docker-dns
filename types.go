package dns

import (
	"strconv"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

type Query dns.Question

type Queries []dns.Question

func (q Query) Fields(v ...interface{}) []interface{} {
	return append(v,
		zap.Stringer("query.type", dns.Type(q.Qtype)),
		zap.Stringer("query.class", dns.Class(q.Qclass)),
		zap.String("query.name", q.Name))
}

func (q Queries) Fields(v ...interface{}) []interface{} {
	var out []interface{}
	for i, item := range q {
		num := strconv.Itoa(i)
		out = append(out,
			zap.Stringer("query."+num+".type", dns.Type(item.Qtype)),
			zap.Stringer("query."+num+".class", dns.Class(item.Qclass)),
			zap.String("query."+num+".name", item.Name))
	}

	return append(v, out...)
}
