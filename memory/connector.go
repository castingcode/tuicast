// Package memory provides an in-memory transport for automation and tests.
package memory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/castingcode/tuicast"
)

var _ tuicast.Connector = (*Connector)(nil)
var _ tuicast.SessionOpener = (*connection)(nil)
var _ tuicast.SessionStream = (*stream)(nil)

// Handler runs the remote side of each in-memory session. Returning closes
// that side of the session.
type Handler func(context.Context, io.ReadWriteCloser)

// Connector creates independent in-memory connections using a shared handler.
type Connector struct {
	handler Handler
}

// NewConnector creates an in-memory connector.
func NewConnector(handler Handler) (*Connector, error) {
	if handler == nil {
		return nil, fmt.Errorf("creating memory connector: handler is required")
	}
	return &Connector{handler: handler}, nil
}

// Connect creates an in-memory connection.
func (c *Connector) Connect(context.Context) (tuicast.SessionOpener, error) {
	return &connection{
		handler: c.handler,
		streams: make(map[*stream]struct{}),
	}, nil
}

type connection struct {
	mu sync.Mutex

	handler Handler
	streams map[*stream]struct{}
	closed  bool
}

func (c *connection) OpenSession(ctx context.Context, _ tuicast.SessionRequest) (tuicast.SessionStream, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("opening memory session: %w", ctx.Err())
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, fmt.Errorf("opening memory session: connection is closed")
	}

	client, remote := net.Pipe()
	handlerCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	opened := &stream{
		Conn:   client,
		cancel: cancel,
		onClose: func(closed *stream) {
			c.mu.Lock()
			delete(c.streams, closed)
			c.mu.Unlock()
		},
	}
	c.streams[opened] = struct{}{}
	go func() {
		defer remote.Close()
		c.handler(handlerCtx, remote)
	}()
	return opened, nil
}

func (c *connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	streams := make([]*stream, 0, len(c.streams))
	for opened := range c.streams {
		streams = append(streams, opened)
	}
	c.mu.Unlock()

	var closeErrors []error
	for _, opened := range streams {
		if err := opened.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("closing memory session: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}

type stream struct {
	net.Conn
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
	onClose   func(*stream)
}

func (s *stream) Resize(_, _ int) error {
	return nil
}

func (s *stream) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		if err := s.Conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			s.closeErr = fmt.Errorf("closing memory stream: %w", err)
		}
		if s.onClose != nil {
			s.onClose(s)
		}
	})
	return s.closeErr
}
