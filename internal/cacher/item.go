package cacher

import (
	"github.com/im-kulikov/docker-dns/internal/broadcast"
	"sync"
	"time"
)

// CacheItem represents a cached DNS record
type CacheItem struct {
	sync.RWMutex

	Domain string
	Expire uint32
	Record []string

	now time.Time
	ext map[string]time.Time
	brd broadcast.Broadcaster
}

// NewItem creates a new CacheItem with the specified domain
func NewItem(domain string, brd broadcast.Broadcaster) *CacheItem {
	return &CacheItem{Domain: domain, brd: brd, now: time.Now(), ext: make(map[string]time.Time)}
}

// AddRecords updates the DNS records and resets the TTL to the minimum value
func (c *CacheItem) AddRecords(records []string, ttl uint32) {
	c.Lock()
	defer c.Unlock()

	if ttl < c.Expire && ttl > 0 || c.Expire == 0 {
		c.Expire = ttl
	}

	toUpdate := make([]string, 0, len(records))
	for _, record := range records {
		if _, ok := c.ext[record]; ok {
			continue
		}

		c.ext[record] = time.Now().Add(time.Hour * 24)
		c.Record = append(c.Record, record)
		toUpdate = append(toUpdate, record)
	}

	c.brd.Broadcast(broadcast.UpdateMessage{ToUpdate: toUpdate})
}

// IsExpired checks if the cache item is expired
func (c *CacheItem) IsExpired() bool {
	return time.Now().After(c.now.Add(time.Duration(c.Expire) * time.Second))
}

// Reset resets the cache item
func (c *CacheItem) Reset() {
	c.Lock()
	defer c.Unlock()

	c.now = time.Now()

	c.Expire = 0 // reset TTL
	c.Record = c.Record[:0]

	toRemove := make([]string, 0, len(c.ext))
	for key, now := range c.ext {
		if c.now.After(now) {
			delete(c.ext, key)

			toRemove = append(toRemove, key)
		}

		c.Record = append(c.Record, key)
	}

	c.brd.Broadcast(broadcast.UpdateMessage{ToRemove: toRemove})
}
