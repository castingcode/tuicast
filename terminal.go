package tuicast

import (
	"io"
)

// TerminalProfile identifies the behavior emulated by a terminal.
type TerminalProfile string

const (
	// ProfileVT220 identifies the DEC VT220 terminal profile.
	ProfileVT220 TerminalProfile = "vt220"
	// ProfileXTerm identifies the xterm terminal profile with 256-color support.
	ProfileXTerm TerminalProfile = "xterm-256color"
)

// Terminal consumes bytes received from a host and maintains terminal state.
// Implementations return detached snapshots that remain stable as more bytes
// are processed.
type Terminal interface {
	io.Writer
	Profile() TerminalProfile
	Resize(width, height int) error
	Snapshot() Screen
}

// TerminalEventType identifies a non-screen event emitted while processing
// host output.
type TerminalEventType string

const (
	EventBell    TerminalEventType = "bell"
	EventEnquiry TerminalEventType = "enquiry"
)

// TerminalEvent reports a terminal control event. Sequence is assigned by the
// session; Data contains the configured answerback for an enquiry.
type TerminalEvent struct {
	Sequence uint64
	Type     TerminalEventType
	Data     string
}

// TerminalEventDrainer exposes events produced synchronously by Terminal.Write.
// The session drains events after each host-output write.
type TerminalEventDrainer interface {
	DrainEvents() []TerminalEvent
}

// AnswerbackTerminal configures the response sent when the host transmits ENQ.
type AnswerbackTerminal interface {
	SetAnswerback(string) error
}
