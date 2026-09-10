// Package workbench records and replays language-neutral terminal workflows.
package workbench

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/castingcode/tuicast"
)

const recordingVersion = "1"

// SessionControl is the exclusive terminal capability consumed by Workbench.
type SessionControl interface {
	Send([]byte) error
	Press(tuicast.Key, ...tuicast.KeyModifier) error
	WaitFor(context.Context, tuicast.ScreenMatcher) (tuicast.Screen, error)
	Screen() tuicast.Screen
	Release()
}

// AcquireControl acquires exclusive control of one driver session.
type AcquireControl func(uint64) (SessionControl, error)

// Session identifies the terminal on which a recording was created.
type Session struct {
	ID       uint64 `json:"id"`
	Terminal string `json:"terminal"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// Recording is a versioned, language-neutral terminal workflow.
type Recording struct {
	Version string  `json:"version"`
	Session Session `json:"session"`
	Steps   []Step  `json:"steps"`
}

// Step is one replayable Workbench operation.
type Step struct {
	Action              string   `json:"action"`
	Text                string   `json:"text,omitempty"`
	Parameter           string   `json:"parameter,omitempty"`
	Key                 string   `json:"key,omitempty"`
	Modifiers           []string `json:"modifiers,omitempty"`
	TimeoutMilliseconds int64    `json:"timeoutMilliseconds,omitempty"`
}

// Manager owns active recordings and retained traces.
type Manager struct {
	mu      sync.Mutex
	acquire AcquireControl
	states  map[uint64]*recordingState
}

type recordingState struct {
	mu        sync.Mutex
	active    bool
	control   SessionControl
	recording Recording
}

// New creates a Workbench recording manager.
func New(acquire AcquireControl) (*Manager, error) {
	if acquire == nil {
		return nil, fmt.Errorf("creating TUICast Workbench: session control acquisition is required")
	}
	return &Manager{acquire: acquire, states: make(map[uint64]*recordingState)}, nil
}

// Start starts a new recording and acquires exclusive session control.
func (m *Manager) Start(session Session) (Recording, error) {
	if session.ID == 0 {
		return Recording{}, fmt.Errorf("starting Workbench recording: session ID is required")
	}
	if session.Terminal == "" || session.Width <= 0 || session.Height <= 0 {
		return Recording{}, fmt.Errorf("starting Workbench recording: terminal profile and positive dimensions are required")
	}
	control, err := m.acquire(session.ID)
	if err != nil {
		return Recording{}, fmt.Errorf("starting Workbench recording: %w", err)
	}

	m.mu.Lock()
	state := m.states[session.ID]
	if state == nil {
		state = &recordingState{}
		m.states[session.ID] = state
	}
	m.mu.Unlock()

	state.mu.Lock()
	if state.active {
		state.mu.Unlock()
		control.Release()
		return Recording{}, fmt.Errorf("starting Workbench recording: session %d is already recording", session.ID)
	}
	state.active = true
	state.control = control
	state.recording = Recording{Version: recordingVersion, Session: session, Steps: []Step{}}
	recording := copyRecording(state.recording)
	state.mu.Unlock()
	return recording, nil
}

// Type sends text and appends a type step. parameter records a placeholder
// instead of the supplied value so secrets do not enter the trace.
func (m *Manager) Type(sessionID uint64, text, parameter string) (Recording, error) {
	if text == "" {
		return Recording{}, fmt.Errorf("typing in Workbench: text is required")
	}
	if parameter != "" && !validParameter(parameter) {
		return Recording{}, fmt.Errorf("typing in Workbench: parameter must contain only letters, digits, underscores, or hyphens")
	}
	state, err := m.activeState(sessionID)
	if err != nil {
		return Recording{}, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active {
		return Recording{}, fmt.Errorf("typing in Workbench: session %d is not recording", sessionID)
	}
	if err := state.control.Send([]byte(text)); err != nil {
		return Recording{}, fmt.Errorf("typing in Workbench: %w", err)
	}
	state.recording.Steps = append(state.recording.Steps, Step{Action: "type", Text: recordedText(text, parameter), Parameter: parameter})
	return copyRecording(state.recording), nil
}

// Press sends a key and appends a press step.
func (m *Manager) Press(sessionID uint64, key tuicast.Key, modifiers ...tuicast.KeyModifier) (Recording, error) {
	state, err := m.activeState(sessionID)
	if err != nil {
		return Recording{}, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active {
		return Recording{}, fmt.Errorf("pressing key in Workbench: session %d is not recording", sessionID)
	}
	if err := state.control.Press(key, modifiers...); err != nil {
		return Recording{}, fmt.Errorf("pressing key in Workbench: %w", err)
	}
	state.recording.Steps = append(state.recording.Steps, Step{Action: "press", Key: string(key), Modifiers: modifierNames(modifiers)})
	return copyRecording(state.recording), nil
}

// AssertText records the selected current-screen text as a replay wait.
func (m *Manager) AssertText(sessionID uint64, text string, timeout time.Duration) (Recording, error) {
	if text == "" {
		return Recording{}, fmt.Errorf("adding Workbench assertion: text is required")
	}
	if timeout.Milliseconds() <= 0 {
		return Recording{}, fmt.Errorf("adding Workbench assertion: timeout must be positive")
	}
	state, err := m.activeState(sessionID)
	if err != nil {
		return Recording{}, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active {
		return Recording{}, fmt.Errorf("adding Workbench assertion: session %d is not recording", sessionID)
	}
	if !strings.Contains(state.control.Screen().Text(), text) {
		return Recording{}, fmt.Errorf("adding Workbench assertion: current screen does not contain %q", text)
	}
	state.recording.Steps = append(state.recording.Steps, Step{
		Action:              "waitForText",
		Text:                text,
		TimeoutMilliseconds: timeout.Milliseconds(),
	})
	return copyRecording(state.recording), nil
}

// Close releases every active recording. It is safe to call repeatedly.
func (m *Manager) Close() {
	m.mu.Lock()
	states := make([]*recordingState, 0, len(m.states))
	for _, state := range m.states {
		states = append(states, state)
	}
	m.mu.Unlock()
	for _, state := range states {
		state.mu.Lock()
		if state.active {
			state.active = false
			control := state.control
			state.control = nil
			state.mu.Unlock()
			control.Release()
			continue
		}
		state.mu.Unlock()
	}
}

// Stop stops recording, releases the session, and retains the trace.
func (m *Manager) Stop(sessionID uint64) (Recording, error) {
	state, err := m.state(sessionID)
	if err != nil {
		return Recording{}, err
	}
	state.mu.Lock()
	if !state.active {
		state.mu.Unlock()
		return Recording{}, fmt.Errorf("stopping Workbench recording: session %d is not recording", sessionID)
	}
	state.active = false
	control := state.control
	state.control = nil
	recording := copyRecording(state.recording)
	state.mu.Unlock()
	control.Release()
	return recording, nil
}

// Recording returns the current or most recently stopped trace.
func (m *Manager) Recording(sessionID uint64) (Recording, bool, error) {
	state, err := m.state(sessionID)
	if err != nil {
		return Recording{}, false, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return copyRecording(state.recording), state.active, nil
}

// Replay replays the retained trace using values for parameterized type steps.
func (m *Manager) Replay(ctx context.Context, sessionID uint64, parameters map[string]string) error {
	state, err := m.state(sessionID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	if state.active {
		state.mu.Unlock()
		return fmt.Errorf("replaying Workbench recording: stop recording session %d first", sessionID)
	}
	recording := copyRecording(state.recording)
	state.mu.Unlock()

	control, err := m.acquire(sessionID)
	if err != nil {
		return fmt.Errorf("replaying Workbench recording: %w", err)
	}
	defer control.Release()
	for index, step := range recording.Steps {
		switch step.Action {
		case "waitForText":
			waitCtx, cancel := context.WithTimeout(ctx, time.Duration(step.TimeoutMilliseconds)*time.Millisecond)
			_, err = control.WaitFor(waitCtx, tuicast.ScreenContains(step.Text))
			cancel()
		case "type":
			text := step.Text
			if step.Parameter != "" {
				var exists bool
				text, exists = parameters[step.Parameter]
				if !exists {
					return fmt.Errorf("replaying Workbench step %d: parameter %q is required", index+1, step.Parameter)
				}
			}
			err = control.Send([]byte(text))
		case "press":
			modifiers, modifierErr := parseModifiers(step.Modifiers)
			if modifierErr != nil {
				err = modifierErr
			} else {
				err = control.Press(tuicast.Key(step.Key), modifiers...)
			}
		default:
			err = fmt.Errorf("unsupported action %q", step.Action)
		}
		if err != nil {
			return fmt.Errorf("replaying Workbench step %d: %w", index+1, err)
		}
	}
	return nil
}

func (m *Manager) activeState(sessionID uint64) (*recordingState, error) {
	state, err := m.state(sessionID)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (m *Manager) state(sessionID uint64) (*recordingState, error) {
	m.mu.Lock()
	state := m.states[sessionID]
	m.mu.Unlock()
	if state == nil {
		return nil, fmt.Errorf("accessing Workbench recording: session %d has no recording", sessionID)
	}
	return state, nil
}

func copyRecording(recording Recording) Recording {
	result := recording
	result.Steps = make([]Step, len(recording.Steps))
	for index, step := range recording.Steps {
		result.Steps[index] = step
		result.Steps[index].Modifiers = append([]string(nil), step.Modifiers...)
	}
	return result
}

func recordedText(text, parameter string) string {
	if parameter != "" {
		return ""
	}
	return text
}

func validParameter(value string) bool {
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return value != ""
}

func modifierNames(modifiers []tuicast.KeyModifier) []string {
	result := make([]string, 0, len(modifiers))
	seen := make(map[string]struct{}, len(modifiers))
	for _, modifier := range modifiers {
		var name string
		switch modifier {
		case tuicast.ModifierShift:
			name = "Shift"
		case tuicast.ModifierAlt:
			name = "Alt"
		case tuicast.ModifierControl:
			name = "Control"
		default:
			name = fmt.Sprintf("Unknown(%d)", modifier)
		}
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	return result
}

func parseModifiers(names []string) ([]tuicast.KeyModifier, error) {
	result := make([]tuicast.KeyModifier, len(names))
	for index, name := range names {
		switch name {
		case "Shift":
			result[index] = tuicast.ModifierShift
		case "Alt", "Meta":
			result[index] = tuicast.ModifierAlt
		case "Control":
			result[index] = tuicast.ModifierControl
		default:
			return nil, fmt.Errorf("unknown key modifier %q", name)
		}
	}
	return result, nil
}
