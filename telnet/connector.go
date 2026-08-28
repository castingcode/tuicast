// Package telnet implements Telnet transport and option negotiation.
package telnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/castingcode/tuicast"
)

var _ tuicast.Connector = (*Connector)(nil)
var _ tuicast.SessionOpener = (*connection)(nil)
var _ tuicast.SessionStream = (*stream)(nil)

// DialContextFunc establishes a network connection.
type DialContextFunc func(context.Context, string, string) (net.Conn, error)

// Config configures a Telnet endpoint.
type Config struct {
	Address     string
	DialContext DialContextFunc
}

// Connector establishes Telnet connections.
type Connector struct {
	address string
	dial    DialContextFunc
}

// NewConnector creates a Telnet connector.
func NewConnector(config Config) (*Connector, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("creating Telnet connector: address is required")
	}
	dial := config.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	return &Connector{address: config.Address, dial: dial}, nil
}

// Connect establishes one Telnet network connection. Telnet does not
// multiplex, so the returned connection permits one session.
func (c *Connector) Connect(ctx context.Context) (tuicast.SessionOpener, error) {
	conn, err := c.dial(ctx, "tcp", c.address)
	if err != nil {
		return nil, fmt.Errorf("dialing Telnet server %s: %w", c.address, err)
	}
	return &connection{conn: conn}, nil
}

type connection struct {
	mu sync.Mutex

	conn   net.Conn
	opened bool
	closed bool
}

func (c *connection) OpenSession(ctx context.Context, request tuicast.SessionRequest) (tuicast.SessionStream, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("opening Telnet session: %w", ctx.Err())
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, fmt.Errorf("opening Telnet session: connection is closed")
	}
	if c.opened {
		return nil, fmt.Errorf("opening Telnet session: connection already has a session")
	}
	if request.Width <= 0 || request.Height <= 0 {
		return nil, fmt.Errorf("opening Telnet session: dimensions must be positive")
	}
	if request.Width > 65535 || request.Height > 65535 {
		return nil, fmt.Errorf("opening Telnet session: dimensions exceed NAWS limits")
	}
	c.opened = true
	return newStream(c.conn, request.TerminalType, request.Width, request.Height), nil
}

func (c *connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	if err := c.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("closing Telnet connection: %w", err)
	}
	return nil
}
