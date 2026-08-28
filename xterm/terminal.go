// Package xterm implements the TUICast xterm terminal profile.
package xterm

import (
	"fmt"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/ansi"
)

var _ tuicast.Terminal = (*Terminal)(nil)

// Terminal maintains the state of an xterm-compatible screen.
type Terminal struct {
	*ansi.Terminal
}

// New creates an xterm terminal with the requested dimensions.
func New(width, height int) (*Terminal, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("creating xterm terminal: dimensions must be positive")
	}
	terminal, err := ansi.New(width, height, ansi.XTerm)
	if err != nil {
		return nil, fmt.Errorf("creating xterm terminal: %w", err)
	}
	return &Terminal{Terminal: terminal}, nil
}
