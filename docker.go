package dns

import (
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/libnetwork/netutils"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/im-kulikov/go-bones/logger"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

type DockerClient interface {
	InspectContainer(string) (*docker.Container, error)
	ListContainers(docker.ListContainersOptions) ([]docker.APIContainers, error)
}

type dockerStore struct {
	cacher Cacher
	client DockerClient
	logger logger.Logger
}

var _ Cacher = (*dockerStore)(nil)

func (d *dockerStore) findContainerByHostname(hostname string) (*docker.Container, error) {
	containers, err := d.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		var item *docker.Container
		if item, err = d.client.InspectContainer(container.ID); err != nil {
			d.logger.Warnw("could not inspect container",
				zap.String("container", container.ID),
				zap.Error(err))

			continue
		}

		if item.Config.Hostname+"." == hostname {
			return item, nil
		}
	}

	return nil, ErrNotFound
}

func (d *dockerStore) fetchAllRecords(query dns.Question) ([]dns.RR, error) {
	containers, err := d.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return nil, err
	}

	var records []dns.RR
	for _, container := range containers {
		var item *docker.Container
		if item, err = d.client.InspectContainer(container.ID); err != nil {
			d.logger.Warnw("could not inspect container",
				zap.Stringer("query", question([]dns.Question{query})),
				zap.String("container", container.ID),
				zap.Error(err))

			continue
		}

		if strings.Count(item.Config.Hostname, ".") < 1 {
			d.logger.Warnw("ignoring container with invalid hostname",
				zap.Stringer("query", question([]dns.Question{query})),
				zap.String("container", container.ID),
				zap.String("hostname", item.Config.Hostname))

			continue
		}

		var ip net.IP
		if ip, err = fetchIPAddress(item); err != nil {
			d.logger.Warnw("could not fetch ip address",
				zap.Stringer("query", question([]dns.Question{query})),
				zap.String("container", container.ID),
				zap.Error(err))

			continue
		}

		rec := &dns.A{
			Hdr: dns.RR_Header{
				Name:   item.Config.Hostname + ".",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			A: ip,
		}

		records = append(records, rec)

		d.cacheResult(dns.Question{
			Name:   rec.Hdr.Name,
			Qtype:  query.Qtype,
			Qclass: query.Qclass,
		}, container.ID, []dns.RR{rec})
	}

	return records, nil
}

func fetchIPAddress(container *docker.Container) (net.IP, error) {
	if container.NetworkSettings.IPAddress != "" {
		return net.ParseIP(container.NetworkSettings.IPAddress), nil
	}

	if container.NetworkSettings.Networks != nil {
		for _, network := range container.NetworkSettings.Networks {
			if network.IPAddress != "" {
				return net.ParseIP(network.IPAddress), nil
			}
		}
	}

	return nil, fmt.Errorf("container %s: %w", container.Name, ErrIPNotFound)
}

func (d *dockerStore) fetchContainerByIP(ip string) (*docker.Container, error) {
	containers, err := d.client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		var item *docker.Container
		if item, err = d.client.InspectContainer(container.ID); err != nil {
			d.logger.Warnw("could not inspect container",
				zap.String("container", container.ID),
				zap.Error(err))

			continue
		}

		if item.NetworkSettings.IPAddress == ip {
			return item, nil
		}

		for _, network := range item.NetworkSettings.Networks {
			if network.IPAddress == ip {
				return item, nil
			}

			d.logger.Warnw("ignore container with invalid ip address",
				zap.String("container.id", container.ID),
				zap.String("container.ip", network.IPAddress),
				zap.String("request.ip", ip))
		}
	}

	return nil, ErrNotFound
}

func (d *dockerStore) cacheResult(query dns.Question, cid string, msg []dns.RR) {
	if d.cacher == nil {
		return
	}

	d.cacher.Set(query, cid, msg)
}

func (d *dockerStore) fetchByIP(ip string, query dns.Question) ([]dns.RR, error) {
	container, err := d.fetchContainerByIP(ip)
	if err != nil {
		return nil, err
	}

	out := []dns.RR{
		&dns.PTR{
			Ptr: container.Config.Hostname + ".",
			Hdr: dns.RR_Header{
				Name:   query.Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
		},
	}

	d.cacheResult(query, container.ID, out)

	return out, nil
}

func (d *dockerStore) fetchByHostname(query dns.Question) ([]dns.RR, error) {
	container, err := d.findContainerByHostname(query.Name)
	if err != nil {
		return nil, err
	}

	ip, err := fetchIPAddress(container)
	if err != nil {
		return nil, err
	}

	out := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   query.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    3600,
			},
			A: ip,
		},
	}

	d.cacheResult(query, container.ID, out)

	return out, nil
}

func (d *dockerStore) Get(query dns.Question) ([]dns.RR, error) {
	switch query.Qtype {
	case dns.TypeA:
		if query.Name == "." {
			return d.fetchAllRecords(query)
		}

		return d.fetchByHostname(query)
	case dns.TypePTR:
		if !strings.Contains(query.Name, ".in-addr.arpa.") {
			return nil, ErrNotFound
		}

		ip := strings.TrimSuffix(query.Name, ".in-addr.arpa.")
		ip = netutils.ReverseIP(ip)

		d.logger.Debugw("reverse ip", zap.String("ip", ip))

		return d.fetchByIP(ip, query)
	default:
		return nil, nil
	}
}

func (d *dockerStore) Set(_ dns.Question, _ string, _ []dns.RR) {}
