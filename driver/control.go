package driver

import (
	"context"
	"fmt"
	"sync"

	"github.com/castingcode/tuicast"
)

// SessionControl grants one caller exclusive input control over a registered
// driver session. Release returns input control to JSON-RPC clients.
type SessionControl struct {
	mu      sync.RWMutex
	server  *Server
	id      uint64
	session *tuicast.Session
	gate    *sync.Mutex
	once    sync.Once
}

// AcquireSessionControl acquires exclusive input control for a session.
func (s *Server) AcquireSessionControl(id uint64) (*SessionControl, error) {
	s.mu.Lock()
	record, exists := s.sessions[id]
	s.mu.Unlock()
	if !exists {
		return nil, fmt.Errorf("acquiring control of session %d: session is not registered", id)
	}
	record.controlGate.Lock()
	defer record.controlGate.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.sessions[id]
	if !exists || current.session != record.session {
		return nil, fmt.Errorf("acquiring control of session %d: session is not registered", id)
	}
	if _, exists := s.controls[id]; exists {
		return nil, fmt.Errorf("acquiring control of session %d: session already has an exclusive controller", id)
	}
	control := &SessionControl{server: s, id: id, session: record.session, gate: record.controlGate}
	s.controls[id] = control
	return control, nil
}

// Send writes terminal input through the controlled session.
func (c *SessionControl) Send(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.active() {
		return fmt.Errorf("sending controlled session input: control is released")
	}
	if err := c.session.Send(data); err != nil {
		return fmt.Errorf("sending controlled session input: %w", err)
	}
	return nil
}

// Press sends a named key through the controlled session.
func (c *SessionControl) Press(key tuicast.Key, modifiers ...tuicast.KeyModifier) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.active() {
		return fmt.Errorf("pressing controlled session key: control is released")
	}
	if err := c.session.Press(key, modifiers...); err != nil {
		return fmt.Errorf("pressing controlled session key: %w", err)
	}
	return nil
}

// WaitFor waits for a screen condition on the controlled session.
func (c *SessionControl) WaitFor(ctx context.Context, matcher tuicast.ScreenMatcher) (tuicast.Screen, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.active() {
		return tuicast.Screen{}, fmt.Errorf("waiting on controlled session: control is released")
	}
	screen, err := c.session.WaitFor(ctx, matcher)
	if err != nil {
		return screen, fmt.Errorf("waiting on controlled session: %w", err)
	}
	return screen, nil
}

// Screen returns the controlled session's current detached screen.
func (c *SessionControl) Screen() tuicast.Screen {
	return c.session.Screen()
}

// Release relinquishes exclusive control. It is idempotent.
func (c *SessionControl) Release() {
	c.once.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.gate.Lock()
		defer c.gate.Unlock()
		c.server.mu.Lock()
		if current := c.server.controls[c.id]; current == c {
			delete(c.server.controls, c.id)
		}
		c.server.mu.Unlock()
	})
}

func (c *SessionControl) active() bool {
	c.server.mu.Lock()
	defer c.server.mu.Unlock()
	return c.server.controls[c.id] == c
}
