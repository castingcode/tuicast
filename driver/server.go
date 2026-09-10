package driver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/castingcode/tuicast"
)

// ConnectorFactory creates concrete transport connectors at the driver
// composition boundary.
type ConnectorFactory func(ConnectionOptions) (tuicast.Connector, error)

// TerminalFactory creates a terminal profile at the driver composition
// boundary.
type TerminalFactory func(profile string, width, height int) (tuicast.Terminal, error)

type connectionRecord struct {
	connection *tuicast.Connection
	protocol   string
	address    string
}

type sessionRecord struct {
	connectionID uint64
	session      *tuicast.Session
	terminal     string
	controlGate  *sync.Mutex
}

type subscriptionRecord struct {
	sessionID uint64
	cancel    context.CancelFunc
	start     chan struct{}
}

// Server owns the JSON-RPC object registry and delegates terminal behavior to
// the TUICast library.
type Server struct {
	mu sync.Mutex

	core             *tuicast.Server
	connectors       ConnectorFactory
	terminals        TerminalFactory
	connections      map[uint64]connectionRecord
	sessions         map[uint64]sessionRecord
	subscriptions    map[uint64]subscriptionRecord
	controls         map[uint64]*SessionControl
	nextConnection   uint64
	nextSession      uint64
	nextSubscription uint64
	context          context.Context
	cancel           context.CancelFunc
	closed           bool
}

// New creates a driver server using injected concrete factories.
func New(logger *slog.Logger, connectors ConnectorFactory, terminals TerminalFactory) (*Server, error) {
	if logger == nil {
		return nil, fmt.Errorf("creating TUICast driver: logger is required")
	}
	if connectors == nil {
		return nil, fmt.Errorf("creating TUICast driver: connector factory is required")
	}
	if terminals == nil {
		return nil, fmt.Errorf("creating TUICast driver: terminal factory is required")
	}
	core, err := tuicast.NewServer(logger)
	if err != nil {
		return nil, fmt.Errorf("creating driver core: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		core:          core,
		connectors:    connectors,
		terminals:     terminals,
		connections:   make(map[uint64]connectionRecord),
		sessions:      make(map[uint64]sessionRecord),
		subscriptions: make(map[uint64]subscriptionRecord),
		controls:      make(map[uint64]*SessionControl),
		context:       ctx,
		cancel:        cancel,
	}, nil
}

// Run serves JSON-RPC 2.0 objects from input and writes responses and
// notifications to output. Requests may complete out of order.
func (s *Server) Run(input io.Reader, output io.Writer) error {
	if input == nil {
		return fmt.Errorf("running TUICast driver: input is required")
	}
	if output == nil {
		return fmt.Errorf("running TUICast driver: output is required")
	}
	writer := newRPCWriter(output)
	requests := make(chan request)
	decodeErrors := make(chan error, 1)
	go decodeRequests(input, requests, decodeErrors)

	var active sync.WaitGroup
	shutdown := make(chan struct{}, 1)
	for {
		select {
		case incoming, open := <-requests:
			if !open {
				if err := s.Close(); err != nil {
					return err
				}
				active.Wait()
				if err := writer.err(); err != nil {
					return err
				}
				decodeErr := <-decodeErrors
				if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
					if err := writer.write(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &responseError{Code: -32700, Message: "parse error"}}); err != nil {
						return errors.Join(fmt.Errorf("decoding driver request: %w", decodeErr), err)
					}
					return fmt.Errorf("decoding driver request: %w", decodeErr)
				}
				return nil
			}
			active.Add(1)
			go func() {
				defer active.Done()
				s.serve(incoming, writer, shutdown)
			}()
		case <-shutdown:
			if err := s.Close(); err != nil {
				return err
			}
			active.Wait()
			if err := writer.err(); err != nil {
				return err
			}
			return nil
		case <-writer.failed:
			if err := s.Close(); err != nil {
				return errors.Join(writer.err(), err)
			}
			active.Wait()
			return writer.err()
		}
	}
}

// Close closes subscriptions, sessions, and connections. It is idempotent.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	for _, subscription := range s.subscriptions {
		subscription.cancel()
	}
	s.subscriptions = make(map[uint64]subscriptionRecord)
	s.controls = make(map[uint64]*SessionControl)
	s.mu.Unlock()

	if err := s.core.Close(); err != nil {
		return fmt.Errorf("closing TUICast driver: %w", err)
	}
	return nil
}

type rpcWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
	failure error
	failed  chan struct{}
	once    sync.Once
}

func newRPCWriter(output io.Writer) *rpcWriter {
	return &rpcWriter{
		encoder: json.NewEncoder(output),
		failed:  make(chan struct{}),
	}
}

func (w *rpcWriter) write(value any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return w.failure
	}
	if err := w.encoder.Encode(value); err != nil {
		w.failure = fmt.Errorf("encoding driver response: %w", err)
		w.once.Do(func() { close(w.failed) })
		return w.failure
	}
	return nil
}

func (w *rpcWriter) err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.failure
}

func decodeRequests(input io.Reader, requests chan<- request, result chan<- error) {
	defer close(requests)
	decoder := json.NewDecoder(input)
	for {
		var incoming request
		if err := decoder.Decode(&incoming); err != nil {
			result <- err
			return
		}
		requests <- incoming
	}
}

func (s *Server) serve(incoming request, writer *rpcWriter, shutdown chan<- struct{}) {
	if incoming.JSONRPC != "2.0" || incoming.Method == "" {
		if len(incoming.ID) > 0 {
			if err := writer.write(response{JSONRPC: "2.0", ID: incoming.ID, Error: &responseError{Code: -32600, Message: "invalid request"}}); err != nil {
				return
			}
		}
		return
	}

	result, responseErr := s.call(incoming.Method, incoming.Params, writer)
	if len(incoming.ID) > 0 {
		if err := writer.write(response{JSONRPC: "2.0", ID: incoming.ID, Result: result, Error: responseErr}); err != nil {
			return
		}
	}
	if (incoming.Method == "session.subscribe" || incoming.Method == "session.subscribeEvents") && responseErr == nil {
		if subscription, ok := result.(map[string]uint64); ok {
			s.startSubscription(subscription["subscriptionId"])
		}
	}
	if incoming.Method == "driver.shutdown" && responseErr == nil {
		select {
		case shutdown <- struct{}{}:
		default:
		}
	}
}

func (s *Server) call(method string, params json.RawMessage, writer *rpcWriter) (any, *responseError) {
	switch method {
	case "driver.ping":
		return map[string]string{"protocolVersion": protocolVersion}, nil
	case "driver.shutdown":
		return map[string]bool{"shuttingDown": true}, nil
	case "connection.open":
		return s.openConnection(params)
	case "connection.close":
		return s.closeConnection(params)
	case "session.open":
		return s.openSession(params)
	case "session.close":
		return s.closeSession(params)
	case "session.send":
		return s.send(params)
	case "session.press":
		return s.press(params)
	case "session.resize":
		return s.resize(params)
	case "session.screen":
		return s.screen(params)
	case "session.wait":
		return s.wait(params)
	case "session.waitForIdle":
		return s.waitForIdle(params)
	case "session.subscribe":
		return s.subscribe(params, writer)
	case "session.subscribeEvents":
		return s.subscribeEvents(params, writer)
	case "session.unsubscribe":
		return s.unsubscribe(params)
	default:
		return nil, &responseError{Code: -32601, Message: "method not found"}
	}
}

func (s *Server) openConnection(data json.RawMessage) (any, *responseError) {
	var options ConnectionOptions
	if err := decodeParams(data, &options); err != nil {
		return nil, err
	}
	if options.ConnectTimeoutMilliseconds < 0 {
		return nil, invalidParams("connectTimeoutMilliseconds cannot be negative")
	}
	if _, err := milliseconds(options.ConnectTimeoutMilliseconds, true); err != nil {
		return nil, invalidParams("invalid connectTimeoutMilliseconds: %v", err)
	}
	connector, err := s.connectors(options)
	if err != nil {
		return nil, applicationError(fmt.Errorf("creating connector: %w", err))
	}
	timeout := 30 * time.Second
	if options.ConnectTimeoutMilliseconds > 0 {
		timeout, _ = milliseconds(options.ConnectTimeoutMilliseconds, false)
	}
	ctx, cancel := context.WithTimeout(s.context, timeout)
	defer cancel()
	connection, err := s.core.Connect(ctx, connector)
	if err != nil {
		return nil, applicationError(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if closeErr := connection.Close(); closeErr != nil {
			return nil, applicationError(fmt.Errorf("closing connection opened during shutdown: %w", closeErr))
		}
		return nil, applicationError(fmt.Errorf("opening connection: driver is closed"))
	}
	s.nextConnection++
	id := s.nextConnection
	s.connections[id] = connectionRecord{
		connection: connection,
		protocol:   options.Protocol,
		address:    options.Address,
	}
	return map[string]uint64{"connectionId": id}, nil
}

func (s *Server) closeConnection(data json.RawMessage) (any, *responseError) {
	var params struct {
		ConnectionID uint64 `json:"connectionId"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	s.mu.Lock()
	record, exists := s.connections[params.ConnectionID]
	if exists {
		delete(s.connections, params.ConnectionID)
		for id, session := range s.sessions {
			if session.connectionID == params.ConnectionID {
				s.cancelSessionSubscriptionsLocked(id)
				delete(s.controls, id)
				delete(s.sessions, id)
			}
		}
	}
	s.mu.Unlock()
	if !exists {
		return nil, invalidParams("unknown connection %d", params.ConnectionID)
	}
	if err := record.connection.Close(); err != nil {
		return nil, applicationError(err)
	}
	return map[string]bool{"closed": true}, nil
}

func (s *Server) openSession(data json.RawMessage) (any, *responseError) {
	var params struct {
		ConnectionID uint64 `json:"connectionId"`
		Terminal     string `json:"terminal"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		Answerback   string `json:"answerback,omitempty"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	s.mu.Lock()
	connection, exists := s.connections[params.ConnectionID]
	s.mu.Unlock()
	if !exists {
		return nil, invalidParams("unknown connection %d", params.ConnectionID)
	}
	terminal, err := s.terminals(params.Terminal, params.Width, params.Height)
	if err != nil {
		return nil, applicationError(fmt.Errorf("creating terminal: %w", err))
	}
	if params.Answerback != "" {
		configurable, ok := terminal.(tuicast.AnswerbackTerminal)
		if !ok {
			return nil, applicationError(fmt.Errorf("configuring answerback: terminal profile %q does not support answerback", params.Terminal))
		}
		if err := configurable.SetAnswerback(params.Answerback); err != nil {
			return nil, applicationError(err)
		}
	}
	session, err := connection.connection.NewSession(s.context, terminal)
	if err != nil {
		return nil, applicationError(err)
	}

	s.mu.Lock()
	current, stillOpen := s.connections[params.ConnectionID]
	if s.closed || !stillOpen || current.connection != connection.connection {
		s.mu.Unlock()
		if closeErr := session.Close(); closeErr != nil {
			return nil, applicationError(fmt.Errorf("closing session opened during connection shutdown: %w", closeErr))
		}
		return nil, applicationError(fmt.Errorf("opening session: connection is closed"))
	}
	s.nextSession++
	id := s.nextSession
	s.sessions[id] = sessionRecord{
		connectionID: params.ConnectionID,
		session:      session,
		terminal:     params.Terminal,
		controlGate:  &sync.Mutex{},
	}
	s.mu.Unlock()
	return map[string]uint64{"sessionId": id}, nil
}

func (s *Server) closeSession(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID uint64 `json:"sessionId"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	s.mu.Lock()
	record, exists := s.sessions[params.SessionID]
	if exists {
		delete(s.sessions, params.SessionID)
		s.cancelSessionSubscriptionsLocked(params.SessionID)
		delete(s.controls, params.SessionID)
	}
	s.mu.Unlock()
	if !exists {
		return nil, invalidParams("unknown session %d", params.SessionID)
	}
	if err := record.session.Close(); err != nil {
		return nil, applicationError(err)
	}
	return map[string]bool{"closed": true}, nil
}

func (s *Server) send(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID uint64  `json:"sessionId"`
		Text      *string `json:"text,omitempty"`
		Base64    *string `json:"base64,omitempty"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	if (params.Text == nil) == (params.Base64 == nil) {
		return nil, invalidParams("exactly one of text or base64 is required")
	}
	var payload []byte
	if params.Text != nil {
		payload = []byte(*params.Text)
	} else {
		decoded, err := base64.StdEncoding.DecodeString(*params.Base64)
		if err != nil {
			return nil, invalidParams("invalid base64 input: %v", err)
		}
		payload = decoded
	}
	session, release, responseErr := s.getSessionForMutation(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	defer release()
	if err := session.Send(payload); err != nil {
		return nil, applicationError(err)
	}
	return map[string]int{"bytesSent": len(payload)}, nil
}

func (s *Server) press(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID uint64   `json:"sessionId"`
		Key       string   `json:"key"`
		Modifiers []string `json:"modifiers,omitempty"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	session, release, responseErr := s.getSessionForMutation(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	defer release()
	modifiers := make([]tuicast.KeyModifier, len(params.Modifiers))
	for index, modifier := range params.Modifiers {
		switch modifier {
		case "Shift":
			modifiers[index] = tuicast.ModifierShift
		case "Alt", "Meta":
			modifiers[index] = tuicast.ModifierAlt
		case "Control":
			modifiers[index] = tuicast.ModifierControl
		default:
			return nil, invalidParams("unknown key modifier %q", modifier)
		}
	}
	if err := session.Press(tuicast.Key(params.Key), modifiers...); err != nil {
		return nil, applicationError(err)
	}
	return map[string]bool{"sent": true}, nil
}

func (s *Server) resize(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID uint64 `json:"sessionId"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	session, release, responseErr := s.getSessionForMutation(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	defer release()
	if err := session.Resize(params.Width, params.Height); err != nil {
		return nil, applicationError(err)
	}
	return map[string]bool{"resized": true}, nil
}

func (s *Server) screen(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID uint64 `json:"sessionId"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	session, responseErr := s.getSession(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	return makeScreenResult(session.Screen()), nil
}

func (s *Server) wait(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID           uint64  `json:"sessionId"`
		Matcher             Matcher `json:"matcher"`
		TimeoutMilliseconds int64   `json:"timeoutMilliseconds"`
		StableMilliseconds  int64   `json:"stableMilliseconds,omitempty"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	if params.TimeoutMilliseconds <= 0 {
		return nil, invalidParams("timeoutMilliseconds must be positive")
	}
	if params.StableMilliseconds < 0 {
		return nil, invalidParams("stableMilliseconds cannot be negative")
	}
	matcher, err := makeMatcher(params.Matcher)
	if err != nil {
		return nil, invalidParams("invalid matcher: %v", err)
	}
	session, responseErr := s.getSession(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	timeout, err := milliseconds(params.TimeoutMilliseconds, false)
	if err != nil {
		return nil, invalidParams("invalid timeoutMilliseconds: %v", err)
	}
	stable, err := milliseconds(params.StableMilliseconds, true)
	if err != nil {
		return nil, invalidParams("invalid stableMilliseconds: %v", err)
	}
	ctx, cancel := context.WithTimeout(s.context, timeout)
	defer cancel()
	var screen tuicast.Screen
	if params.StableMilliseconds > 0 {
		screen, err = session.WaitForStable(ctx, matcher, stable)
	} else {
		screen, err = session.WaitFor(ctx, matcher)
	}
	if err != nil {
		return nil, applicationError(err)
	}
	return makeScreenResult(screen), nil
}

func (s *Server) waitForIdle(data json.RawMessage) (any, *responseError) {
	var params struct {
		SessionID           uint64 `json:"sessionId"`
		TimeoutMilliseconds int64  `json:"timeoutMilliseconds"`
		QuietMilliseconds   int64  `json:"quietMilliseconds"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	if params.TimeoutMilliseconds <= 0 {
		return nil, invalidParams("timeoutMilliseconds must be positive")
	}
	if params.QuietMilliseconds <= 0 {
		return nil, invalidParams("quietMilliseconds must be positive")
	}
	session, responseErr := s.getSession(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	timeout, err := milliseconds(params.TimeoutMilliseconds, false)
	if err != nil {
		return nil, invalidParams("invalid timeoutMilliseconds: %v", err)
	}
	quiet, err := milliseconds(params.QuietMilliseconds, false)
	if err != nil {
		return nil, invalidParams("invalid quietMilliseconds: %v", err)
	}
	ctx, cancel := context.WithTimeout(s.context, timeout)
	defer cancel()
	screen, err := session.WaitForIdle(ctx, quiet)
	if err != nil {
		return nil, applicationError(err)
	}
	return makeScreenResult(screen), nil
}

func (s *Server) subscribe(data json.RawMessage, writer *rpcWriter) (any, *responseError) {
	var params struct {
		SessionID uint64 `json:"sessionId"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	session, responseErr := s.getSession(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	ctx, cancel := context.WithCancel(s.context)
	start := make(chan struct{})
	s.mu.Lock()
	s.nextSubscription++
	id := s.nextSubscription
	s.subscriptions[id] = subscriptionRecord{sessionID: params.SessionID, cancel: cancel, start: start}
	s.mu.Unlock()

	go func() {
		select {
		case <-start:
		case <-ctx.Done():
			return
		}
		for screen := range session.Screens(ctx) {
			if err := writer.write(notification{
				JSONRPC: "2.0",
				Method:  "session.screen",
				Params: map[string]any{
					"subscriptionId": id,
					"sessionId":      params.SessionID,
					"screen":         makeScreenResult(screen),
				},
			}); err != nil {
				cancel()
				break
			}
		}
		s.mu.Lock()
		delete(s.subscriptions, id)
		s.mu.Unlock()
	}()
	return map[string]uint64{"subscriptionId": id}, nil
}

func (s *Server) subscribeEvents(data json.RawMessage, writer *rpcWriter) (any, *responseError) {
	var params struct {
		SessionID uint64 `json:"sessionId"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	session, responseErr := s.getSession(params.SessionID)
	if responseErr != nil {
		return nil, responseErr
	}
	ctx, cancel := context.WithCancel(s.context)
	start := make(chan struct{})
	s.mu.Lock()
	s.nextSubscription++
	id := s.nextSubscription
	s.subscriptions[id] = subscriptionRecord{sessionID: params.SessionID, cancel: cancel, start: start}
	s.mu.Unlock()
	events := session.Events(ctx)

	go func() {
		select {
		case <-start:
		case <-ctx.Done():
			return
		}
		for event := range events {
			if err := writer.write(notification{
				JSONRPC: "2.0",
				Method:  "session.event",
				Params: map[string]any{
					"subscriptionId": id,
					"sessionId":      params.SessionID,
					"event": terminalEventResult{
						Sequence: event.Sequence,
						Type:     string(event.Type),
						Data:     event.Data,
					},
				},
			}); err != nil {
				cancel()
				break
			}
		}
		s.mu.Lock()
		delete(s.subscriptions, id)
		s.mu.Unlock()
	}()
	return map[string]uint64{"subscriptionId": id}, nil
}

func (s *Server) startSubscription(id uint64) {
	s.mu.Lock()
	record, exists := s.subscriptions[id]
	s.mu.Unlock()
	if exists {
		close(record.start)
	}
}

func (s *Server) unsubscribe(data json.RawMessage) (any, *responseError) {
	var params struct {
		SubscriptionID uint64 `json:"subscriptionId"`
	}
	if err := decodeParams(data, &params); err != nil {
		return nil, err
	}
	s.mu.Lock()
	record, exists := s.subscriptions[params.SubscriptionID]
	if exists {
		delete(s.subscriptions, params.SubscriptionID)
	}
	s.mu.Unlock()
	if !exists {
		return nil, invalidParams("unknown subscription %d", params.SubscriptionID)
	}
	record.cancel()
	return map[string]bool{"unsubscribed": true}, nil
}

func (s *Server) getSession(id uint64) (*tuicast.Session, *responseError) {
	s.mu.Lock()
	record, exists := s.sessions[id]
	s.mu.Unlock()
	if !exists {
		return nil, invalidParams("unknown session %d", id)
	}
	return record.session, nil
}

func (s *Server) getSessionForMutation(id uint64) (*tuicast.Session, func(), *responseError) {
	s.mu.Lock()
	record, exists := s.sessions[id]
	s.mu.Unlock()
	if !exists {
		return nil, nil, invalidParams("unknown session %d", id)
	}
	record.controlGate.Lock()
	s.mu.Lock()
	current, stillRegistered := s.sessions[id]
	_, controlled := s.controls[id]
	s.mu.Unlock()
	if !stillRegistered || current.session != record.session {
		record.controlGate.Unlock()
		return nil, nil, invalidParams("unknown session %d", id)
	}
	if controlled {
		record.controlGate.Unlock()
		return nil, nil, applicationError(fmt.Errorf("controlling session %d: session has an exclusive controller", id))
	}
	return record.session, record.controlGate.Unlock, nil
}

func (s *Server) cancelSessionSubscriptionsLocked(sessionID uint64) {
	for id, subscription := range s.subscriptions {
		if subscription.sessionID == sessionID {
			subscription.cancel()
			delete(s.subscriptions, id)
		}
	}
}

func makeMatcher(specification Matcher) (tuicast.ScreenMatcher, error) {
	return makeMatcherAtDepth(specification, 0)
}

func makeMatcherAtDepth(specification Matcher, depth int) (tuicast.ScreenMatcher, error) {
	if depth > 64 {
		return nil, fmt.Errorf("matcher nesting exceeds 64 levels")
	}
	count := 0
	if specification.Contains != nil {
		count++
	}
	if specification.Line != nil {
		count++
	}
	if specification.Cursor != nil {
		count++
	}
	if specification.All != nil {
		count++
	}
	if specification.Any != nil {
		count++
	}
	if specification.Not != nil {
		count++
	}
	if count != 1 {
		return nil, fmt.Errorf("exactly one matcher expression is required")
	}

	switch {
	case specification.Contains != nil:
		return tuicast.ScreenContains(*specification.Contains), nil
	case specification.Line != nil:
		return tuicast.ScreenLineEquals(specification.Line.Row, specification.Line.Text), nil
	case specification.Cursor != nil:
		return tuicast.CursorAt(specification.Cursor.Column, specification.Cursor.Row), nil
	case specification.Not != nil:
		matcher, err := makeMatcherAtDepth(*specification.Not, depth+1)
		if err != nil {
			return nil, err
		}
		return tuicast.Not(matcher), nil
	case specification.All != nil:
		if len(specification.All) == 0 {
			return nil, fmt.Errorf("all requires at least one child")
		}
		matchers := make([]tuicast.ScreenMatcher, len(specification.All))
		for index, child := range specification.All {
			matcher, err := makeMatcherAtDepth(child, depth+1)
			if err != nil {
				return nil, err
			}
			matchers[index] = matcher
		}
		return tuicast.AllOf(matchers...), nil
	case specification.Any != nil:
		if len(specification.Any) == 0 {
			return nil, fmt.Errorf("any requires at least one child")
		}
		matchers := make([]tuicast.ScreenMatcher, len(specification.Any))
		for index, child := range specification.Any {
			matcher, err := makeMatcherAtDepth(child, depth+1)
			if err != nil {
				return nil, err
			}
			matchers[index] = matcher
		}
		return tuicast.AnyOf(matchers...), nil
	default:
		return nil, fmt.Errorf("unsupported matcher")
	}
}

func milliseconds(value int64, allowZero bool) (time.Duration, error) {
	if value < 0 || (!allowZero && value == 0) {
		return 0, fmt.Errorf("value must be positive")
	}
	if value > int64(^uint64(0)>>1)/int64(time.Millisecond) {
		return 0, fmt.Errorf("value is too large")
	}
	return time.Duration(value) * time.Millisecond, nil
}

func makeScreenResult(screen tuicast.Screen) screenResult {
	cells := make([]cellResult, len(screen.Cells))
	for index, cell := range screen.Cells {
		cells[index] = cellResult{
			Text:       cell.Text,
			Width:      cell.Width,
			Foreground: int16(cell.Foreground),
			Background: int16(cell.Background),
			Attributes: uint16(cell.Attributes),
		}
	}
	return screenResult{
		Width:  screen.Width,
		Height: screen.Height,
		Cells:  cells,
		Cursor: cursorResult{
			Column:  screen.Cursor.Column,
			Row:     screen.Cursor.Row,
			Visible: screen.Cursor.Visible,
		},
		Revision: screen.Revision,
		Text:     screen.Text(),
	}
}
