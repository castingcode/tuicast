package driver

import (
	"context"
	"fmt"
	"sort"

	"github.com/castingcode/tuicast"
)

// Snapshot is a detached, UI-safe view of the driver's current object graph.
type Snapshot struct {
	State       string               `json:"state"`
	Connections []ConnectionSnapshot `json:"connections"`
	Sessions    []SessionSnapshot    `json:"sessions"`
}

// ConnectionSnapshot contains non-sensitive connection metadata.
type ConnectionSnapshot struct {
	ID       uint64 `json:"id"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
}

// SessionSnapshot describes one registered terminal session.
type SessionSnapshot struct {
	ID           uint64 `json:"id"`
	ConnectionID uint64 `json:"connectionId"`
	Terminal     string `json:"terminal"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Revision     uint64 `json:"revision"`
	State        string `json:"state"`
	Error        string `json:"error,omitempty"`
}

type snapshotSession struct {
	id           uint64
	connectionID uint64
	terminal     string
	session      *tuicast.Session
}

// DriverSnapshot returns a detached view that never contains connection
// credentials or terminal screen contents.
func (s *Server) DriverSnapshot() Snapshot {
	s.mu.Lock()
	state := "running"
	if s.closed {
		state = "closed"
	}
	connections := make([]ConnectionSnapshot, 0, len(s.connections))
	for id, record := range s.connections {
		connections = append(connections, ConnectionSnapshot{
			ID:       id,
			Protocol: record.protocol,
			Address:  record.address,
		})
	}
	sessions := make([]snapshotSession, 0, len(s.sessions))
	for id, record := range s.sessions {
		sessions = append(sessions, snapshotSession{
			id:           id,
			connectionID: record.connectionID,
			terminal:     record.terminal,
			session:      record.session,
		})
	}
	s.mu.Unlock()

	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].id < sessions[j].id })

	result := Snapshot{
		State:       state,
		Connections: connections,
		Sessions:    make([]SessionSnapshot, 0, len(sessions)),
	}
	for _, registered := range sessions {
		screen := registered.session.Screen()
		session := SessionSnapshot{
			ID:           registered.id,
			ConnectionID: registered.connectionID,
			Terminal:     registered.terminal,
			Width:        screen.Width,
			Height:       screen.Height,
			Revision:     screen.Revision,
			State:        "active",
		}
		select {
		case <-registered.session.Done():
			session.State = "ended"
			if err := registered.session.Err(); err != nil {
				session.Error = err.Error()
			}
		default:
		}
		result.Sessions = append(result.Sessions, session)
	}
	return result
}

// SessionScreens observes detached snapshots for a registered session.
func (s *Server) SessionScreens(ctx context.Context, id uint64) (<-chan tuicast.Screen, error) {
	s.mu.Lock()
	record, exists := s.sessions[id]
	s.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("observing driver session %d: session is not registered", id)
	}
	return record.session.Screens(ctx), nil
}
