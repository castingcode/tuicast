// Package mcpserver exposes TUICast terminal automation as MCP tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/workbench"
)

const (
	defaultWaitTimeout = 10 * time.Second
	maximumWaitTimeout = 2 * time.Minute
)

// TerminalFactory creates the terminal configured for a connection profile.
type TerminalFactory func(tuicast.TerminalProfile, int, int) (tuicast.Terminal, error)

// Profile is one operator-approved terminal endpoint available to MCP clients.
// Connector may contain resolved credentials; those values are never exposed by
// the service or its MCP tools.
type Profile struct {
	Name        string
	Description string
	Protocol    string
	Terminal    tuicast.TerminalProfile
	Width       int
	Height      int
	Connector   tuicast.Connector
}

// ProfileInfo is the non-sensitive profile metadata returned to MCP clients.
type ProfileInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Protocol    string `json:"protocol"`
	Terminal    string `json:"terminal"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// ConnectionInfo identifies an opened TUICast connection.
type ConnectionInfo struct {
	ConnectionID uint64 `json:"connectionId"`
	Profile      string `json:"profile"`
}

// SessionInfo identifies an opened TUICast terminal session.
type SessionInfo struct {
	SessionID    uint64 `json:"sessionId"`
	ConnectionID uint64 `json:"connectionId"`
	Terminal     string `json:"terminal"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// ScreenInfo is a detached text representation of a terminal screen.
type ScreenInfo struct {
	SessionID uint64     `json:"sessionId"`
	Text      string     `json:"text"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	Cursor    CursorInfo `json:"cursor"`
	Revision  uint64     `json:"revision"`
}

// CursorInfo is the zero-based cursor location and visibility state.
type CursorInfo struct {
	Column  int  `json:"column"`
	Row     int  `json:"row"`
	Visible bool `json:"visible"`
}

// CloseInfo reports whether a live object was closed. Closing an already
// absent object is successful and returns false.
type CloseInfo struct {
	Closed bool `json:"closed"`
}

type managedConnection struct {
	connection *tuicast.Connection
	profile    Profile
}

type managedSession struct {
	session *tuicast.Session
	info    SessionInfo
}

// Service owns terminal connections and sessions used by one MCP server.
type Service struct {
	mu sync.RWMutex

	server      *tuicast.Server
	terminal    TerminalFactory
	profiles    map[string]Profile
	connections map[uint64]managedConnection
	sessions    map[uint64]managedSession
	recordings  map[uint64]*workbench.Recording
	closed      bool
	closeOnce   sync.Once
	closeErr    error
}

// New creates an MCP terminal automation service.
func New(logger *slog.Logger, profiles []Profile, terminal TerminalFactory) (*Service, error) {
	if terminal == nil {
		return nil, fmt.Errorf("creating MCP service: terminal factory is required")
	}
	server, err := tuicast.NewServer(logger)
	if err != nil {
		return nil, fmt.Errorf("creating MCP service: %w", err)
	}
	available := make(map[string]Profile, len(profiles))
	for _, profile := range profiles {
		if err := validateProfile(profile); err != nil {
			return nil, errors.Join(err, server.Close())
		}
		if _, exists := available[profile.Name]; exists {
			return nil, errors.Join(fmt.Errorf("creating MCP service: duplicate profile %q", profile.Name), server.Close())
		}
		available[profile.Name] = profile
	}
	if len(available) == 0 {
		return nil, errors.Join(fmt.Errorf("creating MCP service: at least one profile is required"), server.Close())
	}
	return &Service{
		server:      server,
		terminal:    terminal,
		profiles:    available,
		connections: make(map[uint64]managedConnection),
		sessions:    make(map[uint64]managedSession),
		recordings:  make(map[uint64]*workbench.Recording),
	}, nil
}

func validateProfile(profile Profile) error {
	if profile.Name == "" {
		return fmt.Errorf("creating MCP service: profile name is required")
	}
	if strings.TrimSpace(profile.Name) != profile.Name {
		return fmt.Errorf("creating MCP service: profile name %q has leading or trailing whitespace", profile.Name)
	}
	if profile.Protocol == "" {
		return fmt.Errorf("creating MCP service: profile %q protocol is required", profile.Name)
	}
	if profile.Terminal == "" {
		return fmt.Errorf("creating MCP service: profile %q terminal is required", profile.Name)
	}
	if profile.Width <= 0 || profile.Height <= 0 {
		return fmt.Errorf("creating MCP service: profile %q dimensions must be positive", profile.Name)
	}
	if profile.Connector == nil {
		return fmt.Errorf("creating MCP service: profile %q connector is required", profile.Name)
	}
	return nil
}

// Profiles returns sorted, non-sensitive metadata for approved endpoints.
func (s *Service) Profiles() []ProfileInfo {
	s.mu.RLock()
	profiles := make([]ProfileInfo, 0, len(s.profiles))
	for _, profile := range s.profiles {
		profiles = append(profiles, ProfileInfo{
			Name: profile.Name, Description: profile.Description,
			Protocol: profile.Protocol, Terminal: string(profile.Terminal),
			Width: profile.Width, Height: profile.Height,
		})
	}
	s.mu.RUnlock()
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles
}

// Connect opens an operator-approved named profile.
func (s *Service) Connect(ctx context.Context, profileName string) (ConnectionInfo, error) {
	s.mu.RLock()
	profile, ok := s.profiles[profileName]
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return ConnectionInfo{}, fmt.Errorf("connecting MCP profile: service is closed")
	}
	if !ok {
		return ConnectionInfo{}, fmt.Errorf("connecting MCP profile: unknown profile %q", profileName)
	}
	connection, err := s.server.Connect(ctx, profile.Connector)
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("connecting MCP profile %q: %w", profileName, err)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ConnectionInfo{}, errors.Join(fmt.Errorf("connecting MCP profile: service is closed"), connection.Close())
	}
	s.connections[connection.ID()] = managedConnection{connection: connection, profile: profile}
	s.mu.Unlock()
	return ConnectionInfo{ConnectionID: connection.ID(), Profile: profileName}, nil
}

// OpenSession opens a terminal using the selected profile's fixed terminal and dimensions.
func (s *Service) OpenSession(ctx context.Context, connectionID uint64) (SessionInfo, error) {
	s.mu.RLock()
	managed, ok := s.connections[connectionID]
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return SessionInfo{}, fmt.Errorf("opening MCP session: service is closed")
	}
	if !ok {
		return SessionInfo{}, fmt.Errorf("opening MCP session: unknown connection %d", connectionID)
	}
	terminal, err := s.terminal(managed.profile.Terminal, managed.profile.Width, managed.profile.Height)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("opening MCP session: %w", err)
	}
	session, err := managed.connection.NewSession(ctx, terminal)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("opening MCP session on connection %d: %w", connectionID, err)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return SessionInfo{}, errors.Join(fmt.Errorf("opening MCP session: service is closed"), session.Close())
	}
	current, connected := s.connections[connectionID]
	if !connected || current.connection != managed.connection {
		s.mu.Unlock()
		return SessionInfo{}, errors.Join(fmt.Errorf("opening MCP session: connection %d was closed", connectionID), session.Close())
	}
	info := SessionInfo{
		SessionID: session.ID(), ConnectionID: connectionID,
		Terminal: string(managed.profile.Terminal), Width: managed.profile.Width, Height: managed.profile.Height,
	}
	s.sessions[session.ID()] = managedSession{session: session, info: info}
	s.mu.Unlock()
	return info, nil
}

// Screen returns the current detached screen for a session.
func (s *Service) Screen(sessionID uint64) (ScreenInfo, error) {
	session, err := s.findSession(sessionID)
	if err != nil {
		return ScreenInfo{}, err
	}
	return screenInfo(sessionID, session.Screen()), nil
}

// WaitForText waits until exact text appears on a session screen.
func (s *Service) WaitForText(ctx context.Context, sessionID uint64, text string, timeout time.Duration) (ScreenInfo, error) {
	if text == "" {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen text: text is required")
	}
	timeout, err := effectiveTimeout(timeout)
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen text: %w", err)
	}
	session, err := s.findSession(sessionID)
	if err != nil {
		return ScreenInfo{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	screen, err := session.WaitFor(waitCtx, tuicast.ScreenContains(text))
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen text in session %d: %w", sessionID, err)
	}
	s.record(sessionID, workbench.Step{Action: "waitForText", Text: text, TimeoutMilliseconds: timeout.Milliseconds()})
	return screenInfo(sessionID, screen), nil
}

// Wait waits until matcher succeeds on a session screen. A positive stable
// period also requires that no host output arrives for that period.
func (s *Service) Wait(ctx context.Context, sessionID uint64, matcher tuicast.MatcherSpec, timeout, stable time.Duration) (ScreenInfo, error) {
	screenMatcher, err := matcher.ScreenMatcher()
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen: invalid matcher: %w", err)
	}
	timeout, err = effectiveTimeout(timeout)
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen: %w", err)
	}
	if stable < 0 || stable > maximumWaitTimeout {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen: stable period must be between 0 and %s", maximumWaitTimeout)
	}
	session, err := s.findSession(sessionID)
	if err != nil {
		return ScreenInfo{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var screen tuicast.Screen
	if stable > 0 {
		screen, err = session.WaitForStable(waitCtx, screenMatcher, stable)
	} else {
		screen, err = session.WaitFor(waitCtx, screenMatcher)
	}
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for MCP screen in session %d: %w", sessionID, err)
	}
	s.record(sessionID, workbench.Step{
		Action: "wait", Matcher: &matcher,
		TimeoutMilliseconds: timeout.Milliseconds(), StableMilliseconds: stable.Milliseconds(),
	})
	return screenInfo(sessionID, screen), nil
}

// WaitForIdle waits until no host output arrives for quiet.
func (s *Service) WaitForIdle(ctx context.Context, sessionID uint64, quiet, timeout time.Duration) (ScreenInfo, error) {
	if quiet <= 0 || quiet > maximumWaitTimeout {
		return ScreenInfo{}, fmt.Errorf("waiting for idle MCP session: quiet period must be between 1ms and %s", maximumWaitTimeout)
	}
	timeout, err := effectiveTimeout(timeout)
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for idle MCP session: %w", err)
	}
	session, err := s.findSession(sessionID)
	if err != nil {
		return ScreenInfo{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	screen, err := session.WaitForIdle(waitCtx, quiet)
	if err != nil {
		return ScreenInfo{}, fmt.Errorf("waiting for idle MCP session %d: %w", sessionID, err)
	}
	s.record(sessionID, workbench.Step{
		Action: "waitForIdle", QuietMilliseconds: quiet.Milliseconds(), TimeoutMilliseconds: timeout.Milliseconds(),
	})
	return screenInfo(sessionID, screen), nil
}

// Type sends literal text to a terminal session. When the session is
// recording, a non-empty parameter is recorded in place of text.
func (s *Service) Type(sessionID uint64, text, parameter string) error {
	step, err := workbench.TypeStep(text, parameter)
	if err != nil {
		return fmt.Errorf("typing in MCP session: %w", err)
	}
	session, err := s.findSession(sessionID)
	if err != nil {
		return err
	}
	if err := session.Send([]byte(text)); err != nil {
		return fmt.Errorf("typing in MCP session %d: %w", sessionID, err)
	}
	s.record(sessionID, step)
	return nil
}

// Press sends one key with optional modifiers to a terminal session.
func (s *Service) Press(sessionID uint64, key tuicast.Key, modifiers ...tuicast.KeyModifier) error {
	if key == "" {
		return fmt.Errorf("pressing key in MCP session: key is required")
	}
	session, err := s.findSession(sessionID)
	if err != nil {
		return err
	}
	if err := session.Press(key, modifiers...); err != nil {
		return fmt.Errorf("pressing key in MCP session %d: %w", sessionID, err)
	}
	s.record(sessionID, workbench.PressStep(key, modifiers...))
	return nil
}

// StartRecording begins recording successful input and wait operations on a
// session. A session may have only one active recording.
func (s *Service) StartRecording(sessionID uint64) (workbench.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return workbench.Recording{}, fmt.Errorf("starting MCP recording: service is closed")
	}
	managed, ok := s.sessions[sessionID]
	if !ok {
		return workbench.Recording{}, fmt.Errorf("starting MCP recording: unknown session %d", sessionID)
	}
	if s.recordings[sessionID] != nil {
		return workbench.Recording{}, fmt.Errorf("starting MCP recording: session %d is already recording", sessionID)
	}
	recording := &workbench.Recording{
		Version: workbench.RecordingVersion,
		Session: workbench.Session{
			ID: sessionID, Terminal: managed.info.Terminal, Width: managed.info.Width, Height: managed.info.Height,
		},
		Steps: []workbench.Step{},
	}
	s.recordings[sessionID] = recording
	return *recording, nil
}

// StopRecording ends a session's recording and returns its trace.
func (s *Service) StopRecording(sessionID uint64) (workbench.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return workbench.Recording{}, fmt.Errorf("stopping MCP recording: service is closed")
	}
	if _, ok := s.sessions[sessionID]; !ok {
		return workbench.Recording{}, fmt.Errorf("stopping MCP recording: unknown session %d", sessionID)
	}
	recording := s.recordings[sessionID]
	if recording == nil {
		return workbench.Recording{}, fmt.Errorf("stopping MCP recording: session %d is not recording", sessionID)
	}
	delete(s.recordings, sessionID)
	return *recording, nil
}

func (s *Service) record(sessionID uint64, step workbench.Step) {
	s.mu.Lock()
	if recording := s.recordings[sessionID]; recording != nil {
		recording.Steps = append(recording.Steps, step)
	}
	s.mu.Unlock()
}

// CloseSession closes a session. Repeated cleanup succeeds with Closed false.
func (s *Service) CloseSession(sessionID uint64) (CloseInfo, error) {
	s.mu.Lock()
	managed, ok := s.sessions[sessionID]
	if ok {
		delete(s.sessions, sessionID)
		delete(s.recordings, sessionID)
	}
	s.mu.Unlock()
	if !ok {
		return CloseInfo{Closed: false}, nil
	}
	if err := managed.session.Close(); err != nil {
		return CloseInfo{}, fmt.Errorf("closing MCP session %d: %w", sessionID, err)
	}
	return CloseInfo{Closed: true}, nil
}

// CloseConnection closes a connection and its sessions. Repeated cleanup succeeds with Closed false.
func (s *Service) CloseConnection(connectionID uint64) (CloseInfo, error) {
	s.mu.Lock()
	managed, ok := s.connections[connectionID]
	if ok {
		delete(s.connections, connectionID)
		for sessionID, session := range s.sessions {
			if session.info.ConnectionID == connectionID {
				delete(s.sessions, sessionID)
				delete(s.recordings, sessionID)
			}
		}
	}
	s.mu.Unlock()
	if !ok {
		return CloseInfo{Closed: false}, nil
	}
	if err := managed.connection.Close(); err != nil {
		return CloseInfo{}, fmt.Errorf("closing MCP connection %d: %w", connectionID, err)
	}
	return CloseInfo{Closed: true}, nil
}

// Close closes all service-owned sessions and connections. It is idempotent.
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.sessions = make(map[uint64]managedSession)
		s.connections = make(map[uint64]managedConnection)
		s.recordings = make(map[uint64]*workbench.Recording)
		s.mu.Unlock()
		if err := s.server.Close(); err != nil {
			s.closeErr = fmt.Errorf("closing MCP service: %w", err)
		}
	})
	return s.closeErr
}

func (s *Service) findSession(sessionID uint64) (*tuicast.Session, error) {
	s.mu.RLock()
	managed, ok := s.sessions[sessionID]
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return nil, fmt.Errorf("accessing MCP session: service is closed")
	}
	if !ok {
		return nil, fmt.Errorf("accessing MCP session: unknown session %d", sessionID)
	}
	return managed.session, nil
}

// effectiveTimeout applies the default to a zero timeout and enforces the
// maximum.
func effectiveTimeout(timeout time.Duration) (time.Duration, error) {
	if timeout == 0 {
		return defaultWaitTimeout, nil
	}
	if timeout < time.Millisecond || timeout > maximumWaitTimeout {
		return 0, fmt.Errorf("timeout must be between 1ms and %s", maximumWaitTimeout)
	}
	return timeout, nil
}

func screenInfo(sessionID uint64, screen tuicast.Screen) ScreenInfo {
	return ScreenInfo{
		SessionID: sessionID, Text: screen.Text(), Width: screen.Width, Height: screen.Height,
		Cursor:   CursorInfo{Column: screen.Cursor.Column, Row: screen.Cursor.Row, Visible: screen.Cursor.Visible},
		Revision: screen.Revision,
	}
}
