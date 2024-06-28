package dns

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/im-kulikov/go-bones/logger"
)

// RouterConfig allows to configure RouterOS (Mikrotik) static DNS.
type RouterConfig struct {
	Address  string `env:"ADDRESS" default:"192.168.88.1"`
	Enabled  bool   `env:"ENABLED" default:"false"`
	Username string `env:"USERNAME" default:"admin"`
	Password string `env:"PASSWORD" default:"admin"`
}

func (c RouterConfig) Validate(_ context.Context) error {
	switch {
	case c.Address == "":
		return errors.New("empty RouterOS address")
	case c.Username == "":
		return errors.New("empty RouterOS username")
	case c.Password == "":
		return errors.New("empty RouterOS password")
	default:
		return nil
	}
}

func (c RouterConfig) prepareClient() (*client, error) {
	uri, err := url.Parse(c.Address)
	if err != nil {
		return nil, err
	}

	uri.Scheme = "http"

	return &client{
		username: c.Username,
		password: c.Password,

		uri: uri,
		cli: &http.Client{},
		log: logger.Default(),
	}, nil
}
