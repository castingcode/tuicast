package tuicast

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Connector establishes a connection capable of opening terminal sessions.
type Connector interface {
	Connect(context.Context) (SessionOpener, error)
}

// SessionOpener is the transport-specific side of a Connection.
type SessionOpener interface {
	OpenSession(context.Context, SessionRequest) (SessionStream, error)
	Close() error
}

// SessionStream carries one interactive terminal session. Close must unblock
// any pending Read so session shutdown can complete.
type SessionStream interface {
	io.ReadWriteCloser
	Resize(width, height int) error
}

// SessionRequest describes the terminal requested from a remote server.
type SessionRequest struct {
	TerminalType string
	Width        int
	Height       int
}

// Connection is a server-managed connection to a remote terminal server.
type Connection struct {
	mu sync.Mutex

	id        uint64
	server    *Server
	opener    SessionOpener
	sessions  map[*Session]struct{}
	closed    bool
	closeDone chan struct{}
	closeErr  error
}

// ID returns the connection's server-unique identifier.
func (c *Connection) ID() uint64 {
	return c.id
}

// NewSession opens and starts an interactive session on the connection.
func (c *Connection) NewSession(ctx context.Context, terminal Terminal) (*Session, error) {
	if terminal == nil {
		return nil, fmt.Errorf("opening session: terminal is required")
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("opening session: connection is closed")
	}
	c.mu.Unlock()

	screen := terminal.Snapshot()
	stream, err := c.opener.OpenSession(ctx, SessionRequest{
		TerminalType: string(terminal.Profile()),
		Width:        screen.Width,
		Height:       screen.Height,
	})
	if err != nil {
		return nil, fmt.Errorf("opening transport session: %w", err)
	}

	session := newSession(c.server.nextSessionID(), terminal, stream, c.removeSession)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		if err := stream.Close(); err != nil {
			return nil, fmt.Errorf("closing session opened on a closed connection: %w", err)
		}
		return nil, fmt.Errorf("opening session: connection is closed")
	}
	c.sessions[session] = struct{}{}
	c.mu.Unlock()

	session.start()
	c.server.logger.Info("terminal session opened", "connection_id", c.id, "session_id", session.id)
	return session, nil
}

// Close closes every session and then the underlying connection. It is
// idempotent.
func (c *Connection) Close() error {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		<-done
		c.mu.Lock()
		err := c.closeErr
		c.mu.Unlock()
		return err
	}
	c.closed = true
	sessions := make([]*Session, 0, len(c.sessions))
	for session := range c.sessions {
		sessions = append(sessions, session)
	}
	c.mu.Unlock()

	var closeErrors []error
	for _, session := range sessions {
		if err := session.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("closing session %d: %w", session.id, err))
		}
	}
	if err := c.opener.Close(); err != nil {
		closeErrors = append(closeErrors, fmt.Errorf("closing transport connection: %w", err))
	}

	c.mu.Lock()
	c.closeErr = errors.Join(closeErrors...)
	err := c.closeErr
	close(c.closeDone)
	c.mu.Unlock()
	c.server.removeConnection(c)
	c.server.logger.Info("terminal connection closed", "connection_id", c.id)
	return err
}

func (c *Connection) removeSession(session *Session) {
	c.mu.Lock()
	delete(c.sessions, session)
	c.mu.Unlock()
	c.server.logger.Info("terminal session closed", "connection_id", c.id, "session_id", session.id)
}
