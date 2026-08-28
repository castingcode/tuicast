package tuicast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// Server owns terminal connection and session lifecycles.
type Server struct {
	mu sync.Mutex

	logger         *slog.Logger
	connections    map[*Connection]struct{}
	nextConnection uint64
	nextSession    uint64
	closed         bool
	closeDone      chan struct{}
	closeErr       error
}

// NewServer creates a terminal server using logger for lifecycle events.
func NewServer(logger *slog.Logger) (*Server, error) {
	if logger == nil {
		return nil, fmt.Errorf("creating terminal server: logger is required")
	}
	return &Server{
		logger:      logger,
		connections: make(map[*Connection]struct{}),
		closeDone:   make(chan struct{}),
	}, nil
}

// Connect establishes and tracks a transport connection.
func (s *Server) Connect(ctx context.Context, connector Connector) (*Connection, error) {
	if connector == nil {
		return nil, fmt.Errorf("connecting terminal server: connector is required")
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("connecting terminal server: server is closed")
	}
	s.mu.Unlock()

	opener, err := connector.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connecting terminal transport: %w", err)
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		if err := opener.Close(); err != nil {
			return nil, fmt.Errorf("closing connection opened on a closed server: %w", err)
		}
		return nil, fmt.Errorf("connecting terminal server: server is closed")
	}
	s.nextConnection++
	connection := &Connection{
		id:        s.nextConnection,
		server:    s,
		opener:    opener,
		sessions:  make(map[*Session]struct{}),
		closeDone: make(chan struct{}),
	}
	s.connections[connection] = struct{}{}
	s.mu.Unlock()

	s.logger.Info("terminal connection opened", "connection_id", connection.id)
	return connection, nil
}

// Close closes all tracked sessions and connections. It is idempotent.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		s.mu.Lock()
		err := s.closeErr
		s.mu.Unlock()
		return err
	}
	s.closed = true
	connections := make([]*Connection, 0, len(s.connections))
	for connection := range s.connections {
		connections = append(connections, connection)
	}
	s.mu.Unlock()

	var closeErrors []error
	for _, connection := range connections {
		if err := connection.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("closing connection %d: %w", connection.id, err))
		}
	}

	s.mu.Lock()
	s.closeErr = errors.Join(closeErrors...)
	err := s.closeErr
	close(s.closeDone)
	s.mu.Unlock()
	s.logger.Info("terminal server closed")
	return err
}

func (s *Server) nextSessionID() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSession++
	return s.nextSession
}

func (s *Server) removeConnection(connection *Connection) {
	s.mu.Lock()
	delete(s.connections, connection)
	s.mu.Unlock()
}
