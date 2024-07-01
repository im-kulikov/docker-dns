package dns

import "github.com/im-kulikov/docker-dns/internal/cacher"

func (s *server) findDomain(domain string) (int, bool) {
	s.RLock()
	defer s.RUnlock()

	for i, d := range s.cfg.Domains {
		if d == domain {
			return i, true
		}
	}

	return -1, false
}

func (s *server) Get(domain string) (*cacher.CacheItem, bool) {
	if _, ok := s.findDomain(domain); !ok {
		return nil, false
	}

	return s.rec.Get(domain)
}

func (s *server) Set(domain string, item *cacher.CacheItem) bool {
	if !s.rec.Set(domain, item) {
		return false
	}

	if id, ok := s.findDomain(domain); ok {
		s.cfg.Domains[id] = item.Domain
	} else {
		s.cfg.Domains = append(s.cfg.Domains, item.Domain)
	}

	return true
}

func (s *server) Delete(domain string) {
	s.rec.Delete(domain)

	if id, ok := s.findDomain(domain); ok {
		s.cfg.Domains = append(s.cfg.Domains[:id], s.cfg.Domains[id+1:]...)
	}
}

func (s *server) Range(iter cacher.Iter) { s.rec.Range(iter) }

func (s *server) List() map[string]*cacher.CacheItem {
	s.RLock()
	out := make(map[string]*cacher.CacheItem, len(s.cfg.Domains))
	for _, domain := range s.cfg.Domains {
		if item, ok := s.rec.Get(domain); ok {
			out[domain] = item
		} else {
			out[domain] = &cacher.CacheItem{Domain: domain}
		}
	}
	s.RUnlock()

	return out
}
