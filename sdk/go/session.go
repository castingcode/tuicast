package tuicast

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const defaultIdlePeriod = 100 * time.Millisecond

// TerminalProfile identifies a terminal emulation profile.
type TerminalProfile string

const (
	VT220         TerminalProfile = "vt220"
	XTerm256Color TerminalProfile = "xterm-256color"
)

// SessionOption configures a terminal session.
type SessionOption func(*sessionConfig) error

type sessionConfig struct {
	terminal   TerminalProfile
	width      int
	height     int
	answerback string
}

// WithTerminal selects the terminal profile. The default is VT220.
func WithTerminal(profile TerminalProfile) SessionOption {
	return func(config *sessionConfig) error {
		if profile != VT220 && profile != XTerm256Color {
			return fmt.Errorf("configuring terminal profile: unsupported profile %q", profile)
		}
		config.terminal = profile
		return nil
	}
}

// WithSize selects terminal dimensions. The default is 80 by 24.
func WithSize(width, height int) SessionOption {
	return func(config *sessionConfig) error {
		if width <= 0 || height <= 0 {
			return fmt.Errorf("configuring terminal size: width and height must be positive")
		}
		config.width, config.height = width, height
		return nil
	}
}

// WithAnswerback configures printable ASCII sent automatically for ENQ.
func WithAnswerback(answerback string) SessionOption {
	return func(config *sessionConfig) error {
		if len(answerback) > 20 {
			return fmt.Errorf("configuring terminal answerback: value exceeds 20 bytes")
		}
		for _, character := range []byte(answerback) {
			if character < 0x20 || character > 0x7e {
				return fmt.Errorf("configuring terminal answerback: value must be printable ASCII")
			}
		}
		config.answerback = answerback
		return nil
	}
}

// Key identifies a named terminal key or one printable character.
type Key string

const (
	Enter      Key = "Enter"
	Tab        Key = "Tab"
	Backspace  Key = "Backspace"
	Escape     Key = "Escape"
	ArrowUp    Key = "ArrowUp"
	ArrowDown  Key = "ArrowDown"
	ArrowRight Key = "ArrowRight"
	ArrowLeft  Key = "ArrowLeft"
	Home       Key = "Home"
	End        Key = "End"
	Insert     Key = "Insert"
	Delete     Key = "Delete"
	PageUp     Key = "PageUp"
	PageDown   Key = "PageDown"
	F1         Key = "F1"
	F2         Key = "F2"
	F3         Key = "F3"
	F4         Key = "F4"
	F5         Key = "F5"
	F6         Key = "F6"
	F7         Key = "F7"
	F8         Key = "F8"
	F9         Key = "F9"
	F10        Key = "F10"
	F11        Key = "F11"
	F12        Key = "F12"
)

// Modifier identifies a terminal key modifier.
type Modifier string

const (
	Shift   Modifier = "Shift"
	Control Modifier = "Control"
	Alt     Modifier = "Alt"
	Meta    Modifier = "Meta"
)

// Session is an interactive terminal session managed by the driver.
type Session struct {
	connection *Connection
	id         uint64

	closeMu sync.Mutex
	closed  bool
}

// TerminalEventType identifies a terminal event.
type TerminalEventType string

const (
	EventBell    TerminalEventType = "bell"
	EventEnquiry TerminalEventType = "enquiry"
)

// TerminalEvent is a BELL or ENQ event. Data is the configured answerback for ENQ.
type TerminalEvent struct {
	Sequence uint64            `json:"sequence"`
	Type     TerminalEventType `json:"type"`
	Data     string            `json:"data,omitempty"`
}

// ScreenSubscription delivers detached snapshots on Screens. Delivery is
// coalesced: when the consumer is slow, an unread snapshot is replaced by the
// newest one. Screens is closed by Close or when the driver stops.
type ScreenSubscription struct {
	base    subscriptionBase
	Screens <-chan Screen
}

// EventSubscription delivers terminal events in order on Events. To avoid
// blocking the JSON-RPC reader, Events is closed if its buffer fills. It is also
// closed by Close or when the driver stops.
type EventSubscription struct {
	base   subscriptionBase
	Events <-chan TerminalEvent
}

type subscriptionBase struct {
	client        *Driver
	id            uint64
	mu            sync.Mutex
	channelClosed bool
	unsubscribed  bool
	deliverFunc   func(json.RawMessage) bool
	closeFunc     func()
}

func (s *subscriptionBase) deliver(params json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.channelClosed && !s.deliverFunc(params) {
		s.channelClosed = true
		s.closeFunc()
	}
}

func (s *subscriptionBase) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unsubscribed = true
	if !s.channelClosed {
		s.channelClosed = true
		s.closeFunc()
	}
}

func (s *subscriptionBase) close(ctx context.Context) error {
	s.mu.Lock()
	if s.unsubscribed {
		s.mu.Unlock()
		return nil
	}
	s.unsubscribed = true
	if !s.channelClosed {
		s.channelClosed = true
		s.closeFunc()
	}
	s.mu.Unlock()
	s.client.mu.Lock()
	delete(s.client.subscriptions, s.id)
	s.client.mu.Unlock()
	operationContext, cancel := s.client.operationContext(ctx, s.client.defaultTimeout)
	defer cancel()
	if err := s.client.call(operationContext, "session.unsubscribe", map[string]any{"subscriptionId": s.id}, nil); err != nil {
		return fmt.Errorf("unsubscribing session subscription %d: %w", s.id, err)
	}
	return nil
}

// Subscribe subscribes to coalesced screen snapshots, beginning with the current screen.
func (s *Session) Subscribe(ctx context.Context) (*ScreenSubscription, error) {
	channel := make(chan Screen, 1)
	result := &ScreenSubscription{Screens: channel}
	result.base = subscriptionBase{client: s.connection.client, closeFunc: func() { close(channel) }}
	result.base.deliverFunc = func(data json.RawMessage) bool {
		var params struct {
			Screen Screen `json:"screen"`
		}
		if json.Unmarshal(data, &params) != nil {
			return true
		}
		select {
		case <-channel:
		default:
		}
		channel <- params.Screen
		return true
	}
	if err := s.addSubscription(ctx, "session.subscribe", &result.base); err != nil {
		return nil, err
	}
	return result, nil
}

// SubscribeEvents subscribes to BELL and ENQ events. Events are buffered in
// order. If the consumer cannot keep up, Events is closed rather than blocking
// unrelated driver responses; callers should continue reading it promptly.
func (s *Session) SubscribeEvents(ctx context.Context) (*EventSubscription, error) {
	channel := make(chan TerminalEvent, 64)
	result := &EventSubscription{Events: channel}
	result.base = subscriptionBase{client: s.connection.client, closeFunc: func() { close(channel) }}
	result.base.deliverFunc = func(data json.RawMessage) bool {
		var params struct {
			Event TerminalEvent `json:"event"`
		}
		if json.Unmarshal(data, &params) != nil {
			return true
		}
		select {
		case channel <- params.Event:
			return true
		default:
			return false
		}
	}
	if err := s.addSubscription(ctx, "session.subscribeEvents", &result.base); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Session) addSubscription(ctx context.Context, method string, subscription *subscriptionBase) error {
	operationContext, cancel := s.connection.client.operationContext(ctx, s.connection.client.defaultTimeout)
	defer cancel()
	var result struct {
		SubscriptionID uint64 `json:"subscriptionId"`
	}
	if err := s.connection.client.call(operationContext, method, map[string]any{"sessionId": s.id}, &result); err != nil {
		return fmt.Errorf("subscribing to session: %w", err)
	}
	subscription.id = result.SubscriptionID
	s.connection.client.mu.Lock()
	s.connection.client.subscriptions[result.SubscriptionID] = subscription
	queued := s.connection.client.queuedNotifications[result.SubscriptionID]
	delete(s.connection.client.queuedNotifications, result.SubscriptionID)
	s.connection.client.mu.Unlock()
	for _, notification := range queued {
		subscription.deliver(notification)
	}
	return nil
}

// Close unsubscribes and closes Screens. It is idempotent.
func (s *ScreenSubscription) Close(ctx context.Context) error { return s.base.close(ctx) }

// Close unsubscribes and closes Events. It is idempotent.
func (s *EventSubscription) Close(ctx context.Context) error { return s.base.close(ctx) }

// ID returns the driver's session identifier.
func (s *Session) ID() uint64 { return s.id }

// OpenSession opens a VT220 80x24 session unless options override the defaults.
func (c *Connection) OpenSession(ctx context.Context, options ...SessionOption) (*Session, error) {
	config := sessionConfig{terminal: VT220, width: 80, height: 24}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("opening session: session option is required")
		}
		if err := option(&config); err != nil {
			return nil, fmt.Errorf("opening session: %w", err)
		}
	}
	operationContext, cancel := c.client.operationContext(ctx, c.client.defaultTimeout)
	defer cancel()
	var result struct {
		SessionID uint64 `json:"sessionId"`
	}
	if err := c.client.call(operationContext, "session.open", map[string]any{
		"connectionId": c.id,
		"terminal":     config.terminal,
		"width":        config.width,
		"height":       config.height,
		"answerback":   config.answerback,
	}, &result); err != nil {
		return nil, fmt.Errorf("opening session: %w", err)
	}
	return &Session{connection: c, id: result.SessionID}, nil
}

// Type sends UTF-8 text to the terminal session.
func (s *Session) Type(ctx context.Context, text string) error {
	return s.input(ctx, map[string]any{"sessionId": s.id, "text": text})
}

// Send sends arbitrary bytes to the terminal session.
func (s *Session) Send(ctx context.Context, data []byte) error {
	return s.input(ctx, map[string]any{"sessionId": s.id, "base64": base64.StdEncoding.EncodeToString(data)})
}

func (s *Session) input(ctx context.Context, params map[string]any) error {
	operationContext, cancel := s.connection.client.operationContext(ctx, s.connection.client.defaultTimeout)
	defer cancel()
	if err := s.connection.client.call(operationContext, "session.send", params, nil); err != nil {
		return fmt.Errorf("sending session input: %w", err)
	}
	return nil
}

// Press sends a named or printable key with optional modifiers.
func (s *Session) Press(ctx context.Context, key Key, modifiers ...Modifier) error {
	operationContext, cancel := s.connection.client.operationContext(ctx, s.connection.client.defaultTimeout)
	defer cancel()
	if err := s.connection.client.call(operationContext, "session.press", map[string]any{
		"sessionId": s.id,
		"key":       key,
		"modifiers": modifiers,
	}, nil); err != nil {
		return fmt.Errorf("pressing session key %q: %w", key, err)
	}
	return nil
}

// Resize changes the remote and emulated terminal dimensions.
func (s *Session) Resize(ctx context.Context, width, height int) error {
	operationContext, cancel := s.connection.client.operationContext(ctx, s.connection.client.defaultTimeout)
	defer cancel()
	if err := s.connection.client.call(operationContext, "session.resize", map[string]any{
		"sessionId": s.id,
		"width":     width,
		"height":    height,
	}, nil); err != nil {
		return fmt.Errorf("resizing session: %w", err)
	}
	return nil
}

// Screen returns the current detached screen snapshot.
func (s *Session) Screen(ctx context.Context) (Screen, error) {
	operationContext, cancel := s.connection.client.operationContext(ctx, s.connection.client.defaultTimeout)
	defer cancel()
	var screen Screen
	if err := s.connection.client.call(operationContext, "session.screen", map[string]any{"sessionId": s.id}, &screen); err != nil {
		return Screen{}, fmt.Errorf("reading session screen: %w", err)
	}
	return screen, nil
}

// WaitOption configures a screen wait.
type WaitOption func(*waitConfig) error

type waitConfig struct {
	timeout   time.Duration
	stableFor time.Duration
}

// WaitTimeout overrides the client's default timeout for one wait.
func WaitTimeout(timeout time.Duration) WaitOption {
	return func(config *waitConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("configuring wait timeout: duration must be positive")
		}
		config.timeout = timeout
		return nil
	}
}

// StableFor requires matching state followed by a host-output quiet period.
func StableFor(duration time.Duration) WaitOption {
	return func(config *waitConfig) error {
		if duration <= 0 {
			return fmt.Errorf("configuring stable wait: duration must be positive")
		}
		config.stableFor = duration
		return nil
	}
}

// WaitFor waits until a serializable matcher succeeds.
func (s *Session) WaitFor(ctx context.Context, matcher Matcher, options ...WaitOption) (Screen, error) {
	if matcher == nil {
		return Screen{}, fmt.Errorf("waiting for session screen: matcher is required")
	}
	config := waitConfig{timeout: s.connection.client.defaultTimeout}
	for _, option := range options {
		if option == nil {
			return Screen{}, fmt.Errorf("waiting for session screen: wait option is required")
		}
		if err := option(&config); err != nil {
			return Screen{}, fmt.Errorf("waiting for session screen: %w", err)
		}
	}
	timeoutMilliseconds, _ := positiveMilliseconds(config.timeout)
	stableMilliseconds := int64(0)
	if config.stableFor > 0 {
		stableMilliseconds, _ = positiveMilliseconds(config.stableFor)
	}
	operationContext, cancel := s.connection.client.driverOperationContext(ctx, config.timeout)
	defer cancel()
	var screen Screen
	err := s.connection.client.call(operationContext, "session.wait", map[string]any{
		"sessionId":           s.id,
		"matcher":             matcher.driverMatcher(),
		"timeoutMilliseconds": timeoutMilliseconds,
		"stableMilliseconds":  stableMilliseconds,
	}, &screen)
	if err != nil {
		return Screen{}, fmt.Errorf("waiting for session screen: %w", err)
	}
	return screen, nil
}

// WaitForText waits until exact text appears anywhere on the screen.
func (s *Session) WaitForText(ctx context.Context, text string, options ...WaitOption) (Screen, error) {
	return s.WaitFor(ctx, Contains(text), options...)
}

// WaitForTextGone waits until exact text no longer appears on the screen.
func (s *Session) WaitForTextGone(ctx context.Context, text string, options ...WaitOption) (Screen, error) {
	return s.WaitFor(ctx, Not(Contains(text)), options...)
}

// IdleOption configures an idle wait.
type IdleOption func(*idleConfig) error

type idleConfig struct {
	timeout time.Duration
	quiet   time.Duration
}

// IdleFor overrides the default 100 millisecond host-output quiet period.
func IdleFor(duration time.Duration) IdleOption {
	return func(config *idleConfig) error {
		if duration <= 0 {
			return fmt.Errorf("configuring idle period: duration must be positive")
		}
		config.quiet = duration
		return nil
	}
}

// IdleTimeout overrides the client's default timeout for one idle wait.
func IdleTimeout(timeout time.Duration) IdleOption {
	return func(config *idleConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("configuring idle timeout: duration must be positive")
		}
		config.timeout = timeout
		return nil
	}
}

// WaitForIdle waits until no host bytes arrive for the configured quiet period.
func (s *Session) WaitForIdle(ctx context.Context, options ...IdleOption) (Screen, error) {
	config := idleConfig{timeout: s.connection.client.defaultTimeout, quiet: defaultIdlePeriod}
	for _, option := range options {
		if option == nil {
			return Screen{}, fmt.Errorf("waiting for idle session: idle option is required")
		}
		if err := option(&config); err != nil {
			return Screen{}, fmt.Errorf("waiting for idle session: %w", err)
		}
	}
	timeoutMilliseconds, _ := positiveMilliseconds(config.timeout)
	quietMilliseconds, _ := positiveMilliseconds(config.quiet)
	operationContext, cancel := s.connection.client.driverOperationContext(ctx, config.timeout)
	defer cancel()
	var screen Screen
	if err := s.connection.client.call(operationContext, "session.waitForIdle", map[string]any{
		"sessionId":           s.id,
		"timeoutMilliseconds": timeoutMilliseconds,
		"quietMilliseconds":   quietMilliseconds,
	}, &screen); err != nil {
		return Screen{}, fmt.Errorf("waiting for idle session: %w", err)
	}
	return screen, nil
}

// Close closes the session. It is idempotent.
func (s *Session) Close(ctx context.Context) error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	operationContext, cancel := s.connection.client.operationContext(ctx, s.connection.client.defaultTimeout)
	defer cancel()
	if err := s.connection.client.call(operationContext, "session.close", map[string]any{"sessionId": s.id}, nil); err != nil {
		return fmt.Errorf("closing session %d: %w", s.id, err)
	}
	s.closed = true
	return nil
}

// WaitError describes a failed driver-side wait and retains the last screen.
type WaitError struct {
	Kind       string
	Expected   string
	LastScreen Screen
	Cause      error
}

func (e *WaitError) Error() string {
	return fmt.Sprintf("waiting for %s: %s; last screen:\n%s", e.Expected, e.Kind, e.LastScreen.Text())
}

// Unwrap returns the underlying JSON-RPC error.
func (e *WaitError) Unwrap() error { return e.Cause }

// IsTimeout reports whether err is a driver or client-side wait timeout.
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var waitErr *WaitError
	return errors.As(err, &waitErr) && waitErr.Kind == "timeout"
}
