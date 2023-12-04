package dns

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

type Config struct {
	Address string `env:"ADDRESS" default:":53"`
	Network string `env:"NETWORK" default:"udp"`
}

func control(network, address string, c syscall.RawConn) (err error) {
	controlErr := c.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if err != nil {
			return
		}
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if controlErr != nil {
		err = controlErr
	}
	return
}

func (c Config) Validate(ctx context.Context) error {
	var lc net.ListenConfig

	lc.Control = control

	if lis, err := lc.ListenPacket(ctx, c.Network, c.Address); err != nil {
		return err
	} else if err = lis.Close(); err != nil {
		return err
	}

	return nil
}
