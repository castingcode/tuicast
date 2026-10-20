package tuicast

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// WaitError reports why a screen wait ended and retains the last observed
// snapshot for assertion diagnostics. Recording, when present, holds the host
// output that preceded the failure.
type WaitError struct {
	Expected   string
	LastScreen Screen
	Cause      error
	Recording  *FlightRecording
}

func (e *WaitError) Error() string {
	return fmt.Sprintf("waiting for %s: %v; last screen:\n%s", e.Expected, e.Cause, e.LastScreen.Text())
}

// Unwrap returns the context or session error that ended the wait.
func (e *WaitError) Unwrap() error {
	return e.Cause
}

// Session coordinates one transport stream with one terminal.
type Session struct {
	stateMu  sync.Mutex
	sendMu   sync.Mutex
	resizeMu sync.Mutex
	eventMu  sync.Mutex

	id                  uint64
	terminal            Terminal
	stream              SessionStream
	changed             chan struct{}
	done                chan struct{}
	closing             bool
	err                 error
	closeErr            error
	output              uint64
	eventSequence       uint64
	nextEventSubscriber uint64
	eventSubscribers    map[uint64]chan TerminalEvent
	eventHistory        []TerminalEvent
	unsupported         []UnsupportedSequence
	unsupportedIndex    map[string]int
	recorder            *flightRecorder
	closeOnce           sync.Once
	onClose             func(*Session)
	logger              *slog.Logger
}

func newSession(id uint64, terminal Terminal, stream SessionStream, onClose func(*Session), logger *slog.Logger) *Session {
	screen := terminal.Snapshot()
	return &Session{
		id:               id,
		recorder:         newFlightRecorder(time.Now(), screen.Width, screen.Height),
		logger:           logger,
		terminal:         terminal,
		stream:           stream,
		changed:          make(chan struct{}),
		done:             make(chan struct{}),
		eventSubscribers: make(map[uint64]chan TerminalEvent),
		onClose:          onClose,
	}
}

func (s *Session) start() {
	go s.readLoop()
}

// ID returns the session's server-unique identifier.
func (s *Session) ID() uint64 {
	return s.id
}

// Screen returns the terminal's current detached snapshot.
func (s *Session) Screen() Screen {
	return s.terminal.Snapshot()
}

// Send writes input to the remote terminal session. Concurrent sends are
// serialized so their byte streams cannot interleave.
func (s *Session) Send(data []byte) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	s.stateMu.Lock()
	closed := s.closing || s.isDoneLocked()
	s.stateMu.Unlock()
	if closed {
		return fmt.Errorf("sending terminal input: session is closed")
	}

	written := 0
	for written < len(data) {
		n, err := s.stream.Write(data[written:])
		written += n
		if err != nil {
			return fmt.Errorf("sending terminal input: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("sending terminal input: %w", io.ErrShortWrite)
		}
	}
	return nil
}

// Press sends the terminal-profile sequence for a named or printable key with
// optional modifiers.
func (s *Session) Press(key Key, modifiers ...KeyModifier) error {
	press, err := NewKeyPress(key, modifiers...)
	if err != nil {
		return fmt.Errorf("pressing key %q: %w", key, err)
	}
	if encoder, ok := s.terminal.(ModifiedKeyEncoder); ok {
		data, err := encoder.EncodeKeyPress(press)
		if err != nil {
			return fmt.Errorf("pressing key %q: %w", key, err)
		}
		if err := s.Send(data); err != nil {
			return fmt.Errorf("pressing key %q: %w", key, err)
		}
		return nil
	}
	if press.Modifiers != 0 {
		return fmt.Errorf("pressing key %q: terminal profile %q does not encode key modifiers", key, s.terminal.Profile())
	}
	encoder, ok := s.terminal.(KeyEncoder)
	if !ok {
		return fmt.Errorf("pressing key %q: terminal profile %q does not encode named keys", key, s.terminal.Profile())
	}
	data, err := encoder.EncodeKey(key)
	if err != nil {
		return fmt.Errorf("pressing key %q: %w", key, err)
	}
	if err := s.Send(data); err != nil {
		return fmt.Errorf("pressing key %q: %w", key, err)
	}
	return nil
}

// Resize changes both the local terminal and the remote session dimensions.
func (s *Session) Resize(width, height int) error {
	s.resizeMu.Lock()
	defer s.resizeMu.Unlock()

	s.stateMu.Lock()
	closed := s.closing || s.isDoneLocked()
	s.stateMu.Unlock()
	if closed {
		return fmt.Errorf("resizing terminal: session is closed")
	}
	if err := s.stream.Resize(width, height); err != nil {
		return fmt.Errorf("resizing remote terminal: %w", err)
	}
	if err := s.terminal.Resize(width, height); err != nil {
		return fmt.Errorf("resizing local terminal: %w", err)
	}
	s.recorder.resize(time.Now(), width, height)
	s.signalChange()
	return nil
}

// WaitFor waits until matcher accepts a screen, the context expires, or the
// session ends.
func (s *Session) WaitFor(ctx context.Context, matcher ScreenMatcher) (Screen, error) {
	if matcher == nil {
		return Screen{}, fmt.Errorf("waiting for screen: matcher is required")
	}

	for {
		s.stateMu.Lock()
		changed := s.changed
		done := s.done
		err := s.err
		finished := s.isDoneLocked()
		s.stateMu.Unlock()

		screen := s.terminal.Snapshot()
		if matcher.Match(screen) {
			return screen, nil
		}
		if finished {
			if err != nil {
				return screen, s.newWaitError(matcher, screen, fmt.Errorf("session ended: %w", err))
			}
			return screen, s.newWaitError(matcher, screen, fmt.Errorf("session ended"))
		}

		select {
		case <-ctx.Done():
			return screen, s.newWaitError(matcher, screen, ctx.Err())
		case <-changed:
		case <-done:
		}
	}
}

// WaitForStable waits until matcher succeeds and no host output arrives for
// quietPeriod. The matcher is checked again after the quiet period.
func (s *Session) WaitForStable(ctx context.Context, matcher ScreenMatcher, quietPeriod time.Duration) (Screen, error) {
	if matcher == nil {
		return Screen{}, fmt.Errorf("waiting for stable screen: matcher is required")
	}
	if quietPeriod <= 0 {
		return Screen{}, fmt.Errorf("waiting for stable screen: quiet period must be positive")
	}

	for {
		s.stateMu.Lock()
		changed := s.changed
		done := s.done
		err := s.err
		finished := s.isDoneLocked()
		output := s.output
		s.stateMu.Unlock()

		screen := s.terminal.Snapshot()
		if finished {
			if err != nil {
				return screen, s.newWaitError(matcher, screen, fmt.Errorf("session ended: %w", err))
			}
			return screen, s.newWaitError(matcher, screen, fmt.Errorf("session ended"))
		}
		if !matcher.Match(screen) {
			select {
			case <-ctx.Done():
				return screen, s.newWaitError(matcher, screen, ctx.Err())
			case <-changed:
			case <-done:
			}
			continue
		}

		timer := time.NewTimer(quietPeriod)
		select {
		case <-ctx.Done():
			timer.Stop()
			return screen, s.newWaitError(matcher, screen, ctx.Err())
		case <-changed:
			timer.Stop()
			continue
		case <-done:
			timer.Stop()
			continue
		case <-timer.C:
		}

		s.stateMu.Lock()
		currentOutput := s.output
		finished = s.isDoneLocked()
		err = s.err
		s.stateMu.Unlock()
		screen = s.terminal.Snapshot()
		if !finished && currentOutput == output && matcher.Match(screen) {
			return screen, nil
		}
		if finished {
			if err != nil {
				return screen, s.newWaitError(matcher, screen, fmt.Errorf("session ended: %w", err))
			}
			return screen, s.newWaitError(matcher, screen, fmt.Errorf("session ended"))
		}
	}
}

// WaitForIdle waits until no host output arrives for quietPeriod. Host bytes
// that do not visibly change the screen still reset the quiet period.
func (s *Session) WaitForIdle(ctx context.Context, quietPeriod time.Duration) (Screen, error) {
	if quietPeriod <= 0 {
		return Screen{}, fmt.Errorf("waiting for idle terminal: quiet period must be positive")
	}
	matcher := describedScreenMatcher{
		description: fmt.Sprintf("terminal idle for %s", quietPeriod),
		match:       func(Screen) bool { return true },
	}
	return s.WaitForStable(ctx, matcher, quietPeriod)
}

// Screens observes detached snapshots. It sends the current snapshot first,
// then the latest snapshot after each visible screen revision. A slow observer
// may skip intermediate revisions. The channel closes when ctx or the session
// ends.
func (s *Session) Screens(ctx context.Context) <-chan Screen {
	screens := make(chan Screen, 1)
	go func() {
		defer close(screens)

		last := s.terminal.Snapshot()
		if !publishScreen(ctx, screens, last) {
			return
		}
		for {
			s.stateMu.Lock()
			changed := s.changed
			done := s.done
			s.stateMu.Unlock()

			select {
			case <-ctx.Done():
				return
			case <-changed:
			case <-done:
				current := s.terminal.Snapshot()
				if current.Revision != last.Revision {
					publishScreen(ctx, screens, current)
				}
				return
			}

			current := s.terminal.Snapshot()
			if current.Revision != last.Revision {
				last = current
				if !publishScreen(ctx, screens, current) {
					return
				}
			}
		}
	}()
	return screens
}

// Events observes BELL, ENQ, and future non-screen terminal events. Each event
// has a session-monotonic sequence. A subscriber that cannot keep up is closed
// rather than blocking terminal processing.
func (s *Session) Events(ctx context.Context) <-chan TerminalEvent {
	events := make(chan TerminalEvent, 64)
	s.stateMu.Lock()
	if s.isDoneLocked() {
		s.stateMu.Unlock()
		close(events)
		return events
	}
	s.eventMu.Lock()
	s.nextEventSubscriber++
	id := s.nextEventSubscriber
	s.eventSubscribers[id] = events
	s.eventMu.Unlock()
	s.stateMu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
		case <-s.done:
		}
		s.eventMu.Lock()
		if registered, exists := s.eventSubscribers[id]; exists {
			delete(s.eventSubscribers, id)
			close(registered)
		}
		s.eventMu.Unlock()
	}()
	return events
}

// EventsAfter returns the session's retained events whose sequence numbers
// are greater than sequence, oldest first. A session retains its most recent
// 256 events, including events published before any Events subscriber
// existed or that a slow subscriber missed. Sequence numbers are contiguous,
// so a gap before the first returned event shows how many are no longer
// retained.
func (s *Session) EventsAfter(sequence uint64) []TerminalEvent {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	events := make([]TerminalEvent, 0, len(s.eventHistory))
	for _, event := range s.eventHistory {
		if event.Sequence > sequence {
			events = append(events, event)
		}
	}
	return events
}

// Done is closed when the remote session ends or Close completes.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Err returns the error that ended the session, if any.
func (s *Session) Err() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.err
}

// Close closes the stream and waits for its read loop to stop. It is
// idempotent.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.stateMu.Lock()
		s.closing = true
		s.stateMu.Unlock()

		if err := s.stream.Close(); err != nil {
			s.stateMu.Lock()
			s.closeErr = fmt.Errorf("closing terminal stream: %w", err)
			s.stateMu.Unlock()
		}
		<-s.done
	})

	s.stateMu.Lock()
	err := s.closeErr
	s.stateMu.Unlock()
	return err
}

func (s *Session) readLoop() {
	buffer := make([]byte, 32*1024)
	var terminalErr error
	for {
		n, readErr := s.stream.Read(buffer)
		if n > 0 {
			received := time.Now()
			s.signalOutput()
			var err error
			s.recorder.output(received, buffer[:n], func() uint64 {
				_, err = s.terminal.Write(buffer[:n])
				return s.revision()
			})
			if err != nil {
				terminalErr = fmt.Errorf("applying terminal output: %w", err)
				break
			}
			s.signalChange()
			if source, ok := s.terminal.(TerminalEventDrainer); ok {
				for _, event := range source.DrainEvents() {
					if event.Type == EventEnquiry && event.Data != "" {
						if err := s.Send([]byte(event.Data)); err != nil {
							terminalErr = fmt.Errorf("sending terminal answerback: %w", err)
							break
						}
					}
					if event.Type == EventUnsupported {
						if !s.recordUnsupported(event.Data) {
							s.logger.Debug("ignored unsupported terminal sequence", "session_id", s.id, "sequence", event.Data)
							continue
						}
						s.logger.Warn("ignored unsupported terminal sequence", "session_id", s.id, "sequence", event.Data)
					}
					s.publishEvent(event)
				}
				if terminalErr != nil {
					break
				}
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				terminalErr = fmt.Errorf("reading terminal output: %w", readErr)
			}
			break
		}
	}

	if err := s.stream.Close(); err != nil {
		s.stateMu.Lock()
		closing := s.closing
		s.stateMu.Unlock()
		if !closing && terminalErr == nil {
			terminalErr = fmt.Errorf("closing ended terminal stream: %w", err)
		}
	}
	s.finish(terminalErr)
}

func (s *Session) finish(err error) {
	s.stateMu.Lock()
	if s.closing {
		err = nil
	}
	s.err = err
	close(s.changed)
	close(s.done)
	s.eventMu.Lock()
	for id, subscriber := range s.eventSubscribers {
		delete(s.eventSubscribers, id)
		close(subscriber)
	}
	s.eventMu.Unlock()
	s.stateMu.Unlock()
	if s.onClose != nil {
		s.onClose(s)
	}
}

func (s *Session) signalChange() {
	s.stateMu.Lock()
	if !s.isDoneLocked() {
		close(s.changed)
		s.changed = make(chan struct{})
	}
	s.stateMu.Unlock()
}

func (s *Session) signalOutput() {
	s.stateMu.Lock()
	if !s.isDoneLocked() {
		s.output++
		close(s.changed)
		s.changed = make(chan struct{})
	}
	s.stateMu.Unlock()
}

// retainedEvents bounds the history returned by EventsAfter.
const retainedEvents = 256

// trackedUnsupportedSequences bounds the distinct unsupported sequences a
// session counts, because parameterized sequences can vary without limit.
const trackedUnsupportedSequences = 256

// UnsupportedSequence counts how often the host sent a sequence the terminal
// ignored. Sequence uses the notation of EventUnsupported data.
type UnsupportedSequence struct {
	Sequence string
	Count    uint64
}

// UnsupportedSequences returns the sequences the terminal has ignored in the
// order they first appeared, with the number of times each was received. A
// session counts at most 256 distinct sequences; later new sequences are
// neither counted nor reported.
func (s *Session) UnsupportedSequences() []UnsupportedSequence {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	return append([]UnsupportedSequence(nil), s.unsupported...)
}

// recordUnsupported counts an unsupported sequence and reports whether it is
// the first occurrence of a tracked sequence and should be published.
func (s *Session) recordUnsupported(sequence string) bool {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	if index, tracked := s.unsupportedIndex[sequence]; tracked {
		s.unsupported[index].Count++
		return false
	}
	if len(s.unsupported) == trackedUnsupportedSequences {
		return false
	}
	if s.unsupportedIndex == nil {
		s.unsupportedIndex = make(map[string]int)
	}
	s.unsupportedIndex[sequence] = len(s.unsupported)
	s.unsupported = append(s.unsupported, UnsupportedSequence{Sequence: sequence, Count: 1})
	return true
}

func (s *Session) publishEvent(event TerminalEvent) {
	s.eventMu.Lock()
	s.eventSequence++
	event.Sequence = s.eventSequence
	if len(s.eventHistory) == retainedEvents {
		s.eventHistory = append(s.eventHistory[:0], s.eventHistory[1:]...)
	}
	s.eventHistory = append(s.eventHistory, event)
	for id, subscriber := range s.eventSubscribers {
		select {
		case subscriber <- event:
		default:
			delete(s.eventSubscribers, id)
			close(subscriber)
		}
	}
	s.eventMu.Unlock()
}

func (s *Session) newWaitError(matcher ScreenMatcher, screen Screen, cause error) error {
	recording := s.FlightRecording()
	return &WaitError{
		Expected:   matcherDescription(matcher),
		LastScreen: screen,
		Cause:      cause,
		Recording:  &recording,
	}
}

// FlightRecording returns a detached copy of the session's most recent host
// output, up to 64 KiB, with the resizes between reads and the session's
// unsupported-sequence counts. Wait errors include one automatically.
func (s *Session) FlightRecording() FlightRecording {
	return s.recorder.recording(s.terminal.Profile(), time.Now(), s.UnsupportedSequences())
}

func (s *Session) revision() uint64 {
	if reporter, ok := s.terminal.(revisionReporter); ok {
		return reporter.Revision()
	}
	return s.terminal.Snapshot().Revision
}

func publishScreen(ctx context.Context, screens chan Screen, screen Screen) bool {
	select {
	case <-ctx.Done():
		return false
	case screens <- screen:
		return true
	default:
	}

	select {
	case <-screens:
	default:
	}
	select {
	case <-ctx.Done():
		return false
	case screens <- screen:
		return true
	}
}

func (s *Session) isDoneLocked() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}
