package dns

import (
	"github.com/miekg/dns"
)

type chainStore struct {
	stores []Cacher
}

var _ Cacher = (*chainStore)(nil)

func (c *chainStore) Get(query dns.Question) ([]dns.RR, error) {
	for _, store := range c.stores {
		if msg, err := store.Get(query); err == nil {
			return msg, nil
		} else if err != ErrNotFound {
			return nil, err
		}
	}

	return nil, ErrNotFound
}

func (c *chainStore) Set(query dns.Question, cid string, msg []dns.RR) {
	for _, store := range c.stores {
		store.Set(query, cid, msg)
	}
}
