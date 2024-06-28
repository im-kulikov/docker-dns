package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/im-kulikov/go-bones/logger"
)

const (
	cmdList = "/rest/ip/dns/static/print"
	cmdSet  = "/rest/ip/dns/static/set"
)

type CallParam struct {
	Key string
	Val string
}

type client struct {
	uri *url.URL
	cli *http.Client
	log logger.Logger

	username string
	password string
}

func closeIt(log logger.Logger, closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.Errorf("could not close: %s", err)
	}
}

func prepareBody(args ...CallParam) (io.Reader, error) {
	if len(args) == 0 {
		return nil, nil
	}

	tmp := make(map[string]string)
	for _, item := range args {
		tmp[item.Key] = item.Val
	}

	buf := new(bytes.Buffer)

	return buf, json.NewEncoder(buf).Encode(tmp)
}

func (r *client) setLogger(log logger.Logger) { r.log = log }

func (r *client) closeBody(closer io.Closer) { closeIt(r.log, closer) }

func (r *client) setAuth(req *http.Request, err error) (*http.Request, error) {
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(r.username, r.password)

	return req, nil
}

func (r *client) request(ctx context.Context, cmd string, args ...CallParam) (*http.Request, error) {
	out, err := prepareBody(args...)
	if err != nil {
		return nil, err
	}

	return r.setAuth(http.NewRequestWithContext(ctx, http.MethodPost,
		r.uri.JoinPath(cmd).String(), out))
}

type StaticDNSListResponse []struct {
	ID             string `json:".id"`
	Address        string `json:"address,omitempty"`
	Disabled       string `json:"disabled"`
	Dynamic        string `json:"dynamic"`
	Name           string `json:"name,omitempty"`
	TTL            string `json:"ttl"`
	Comment        string `json:"comment,omitempty"`
	ForwardTo      string `json:"forward-to,omitempty"`
	Regexp         string `json:"regexp,omitempty"`
	Type           string `json:"type,omitempty"`
	MatchSubdomain string `json:"match-subdomain,omitempty"`
}

func (r *client) List(ctx context.Context) ([]string, error) {
	var res *http.Response
	if req, err := r.request(ctx, cmdList); err != nil {
		return nil, err
	} else if res, err = r.cli.Do(req); err != nil {
		return nil, err
	}

	defer r.closeBody(res.Body)
	if res.StatusCode < http.StatusOK || res.StatusCode > http.StatusMultipleChoices {
		out, _ := io.ReadAll(res.Body)

		return nil, fmt.Errorf("HTTP Error: %d\n%s", res.StatusCode, string(out))
	}

	buf := new(bytes.Buffer)
	tee := io.TeeReader(res.Body, buf)

	var tmp StaticDNSListResponse
	if err := json.NewDecoder(tee).Decode(&tmp); err != nil {
		return nil, fmt.Errorf("JSON Decode Error: %w\n%s", err, buf.String())
	}

	items := make([]string, 0, len(tmp))
	for _, item := range tmp {
		if item.Comment != "local-dns" {
			continue
		}

		items = append(items, item.ID)
	}

	return items, nil
}

func (r *client) Set(ctx context.Context, ids []string, address string) error {
	args := []CallParam{
		{Key: ".id", Val: strings.Join(ids, ",")},
		{Key: "forward-to", Val: address},
	}

	var res *http.Response
	if req, err := r.request(ctx, cmdSet, args...); err != nil {
		return err
	} else if res, err = r.cli.Do(req); err != nil {
		return err
	}

	defer r.closeBody(res.Body)
	if res.StatusCode < http.StatusOK || res.StatusCode > http.StatusMultipleChoices {
		out, _ := io.ReadAll(res.Body)

		return fmt.Errorf("HTTP Error: %d\n%s", res.StatusCode, string(out))
	}

	return nil
}

func localIP(log logger.Logger) net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatalf("could not fetch local address: %s", err)
	}

	defer closeIt(log, conn)

	localAddress := conn.LocalAddr().(*net.UDPAddr)

	return localAddress.IP
}

func UpdateStaticDNS(top context.Context, log logger.Logger, cfg RouterConfig) error {
	var err error
	if !cfg.Enabled {
		log.Info("RouterOS API disabled")

		return nil
	}

	var cli *client
	if cli, err = cfg.prepareClient(); err != nil {
		return fmt.Errorf("could not prepare RouterOS client: %w", err)
	}

	ctx, cancel := context.WithTimeout(top, time.Second)
	defer cancel()

	var ids []string
	if ids, err = cli.List(ctx); err != nil {
		return fmt.Errorf("could not fetch router OS static DNS: %w", err)
	}

	log.Infof("RouterOS API fetched %d static DNS records", len(ids))

	local := localIP(log).String()

	if err = cli.Set(ctx, ids, local); err != nil {
		return fmt.Errorf("could not update router OS static DNS: %w", err)
	}

	log.Infof("RouterOS API updated %d static DNS records forward-to: %s",
		len(ids), local)

	return nil
}
