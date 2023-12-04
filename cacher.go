package dns

import (
	"context"
	"net"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/netutils"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/im-kulikov/go-bones/service"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

type Cacher interface {
	Get(dns.Question) ([]dns.RR, error)
	Set(dns.Question, string, []dns.RR)
}

type DockerListener interface {
	DockerClient

	Ping() error
	AddEventListener(chan<- *docker.APIEvents) error
}

type CacheWorker interface {
	Cacher

	service.Service
}

type cache struct {
	service.Service

	cli DockerClient
	log logger.Logger
	out chan *docker.APIEvents

	sync.RWMutex
	rec map[dns.Question][]dns.RR
	cnr map[string][]dns.Question
}

func NewCache(cli DockerListener, log logger.Logger) (CacheWorker, error) {
	if err := cli.Ping(); err != nil {
		return nil, err
	}

	out := make(chan *docker.APIEvents)
	if err := cli.AddEventListener(out); err != nil {
		return nil, err
	}

	svc := cache{
		cli: cli,
		log: log,
		out: out,
		cnr: make(map[string][]dns.Question),
		rec: make(map[dns.Question][]dns.RR),
	}

	svc.Service = service.NewWorker("docker-dns-cache", svc.Run)

	return &svc, nil
}

func (c *cache) Get(query dns.Question) ([]dns.RR, error) {
	c.RLock()
	defer c.RUnlock()

	if rec, ok := c.rec[query]; ok {
		msg := new(dns.Msg)
		msg.Answer = rec

		return msg.Copy().Answer, nil
	}

	return nil, ErrNotFound
}

func (c *cache) Set(query dns.Question, cid string, rec []dns.RR) {
	c.Lock()
	defer c.Unlock()

	msg := new(dns.Msg)
	msg.Answer = rec

	c.rec[query] = msg.Copy().Answer

	if _, ok := c.cnr[cid]; !ok {
		c.cnr[cid] = make([]dns.Question, 0)
	}

	c.cnr[cid] = append(c.cnr[cid], query)
}

func (c *cache) handleStart(event *docker.APIEvents) {
	container, err := c.cli.InspectContainer(event.ID)
	if err != nil {
		c.log.Warnw("could not inspect container",
			zap.String("container", event.ID),
			zap.Error(err))

		return
	}

	if strings.Count(container.Config.Hostname, ".") < 1 {
		c.log.Warnw("ignoring container with invalid hostname",
			zap.String("container", container.ID),
			zap.String("hostname", container.Config.Hostname))

		return
	}

	var ipaddr net.IP
	if ipaddr, err = fetchIPAddress(container); err != nil {
		c.log.Warnw("could not fetch ip address",
			zap.String("container", container.ID),
			zap.Error(err))

		return
	}

	c.Set(dns.Question{
		Name:   container.Config.Hostname + ".",
		Qtype:  dns.TypeA,
		Qclass: dns.ClassINET,
	}, container.ID, []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   container.Config.Hostname + ".",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			A: ipaddr,
		},
	})

	c.log.Info("added A record to cache",
		zap.String("container", container.ID),
		zap.String("hostname", container.Config.Hostname))

	revip := netutils.ReverseIP(ipaddr.String())

	c.Set(dns.Question{
		Name:   revip + ".in-addr.arpa.",
		Qtype:  dns.TypePTR,
		Qclass: dns.ClassINET,
	}, container.ID, []dns.RR{
		&dns.PTR{
			Ptr: revip + ".in-addr.arpa.",
			Hdr: dns.RR_Header{
				Name:   container.Config.Hostname + ".",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
		},
	})

	c.log.Info("added PTR record to cache",
		zap.String("container", container.ID),
		zap.String("hostname", container.Config.Hostname))
}

func (c *cache) handleDie(event *docker.APIEvents) {
	if queries, ok := c.cnr[event.ID]; ok {
		for _, query := range queries {
			delete(c.rec, query)

			c.log.Debugw("removed record from cache",
				zap.String("container", event.ID),
				zap.String("hostname", query.Name))
		}
	}
}

func (c *cache) handleEvent(event *docker.APIEvents) {
	switch event.Action {
	case "start":
		c.handleStart(event)
	case "die":
		c.handleDie(event)
	}
}

func (c *cache) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-c.out:
			c.handleEvent(event)
		}
	}
}
