// Package vt220 implements the TUICast DEC VT220 terminal profile.
package vt220

import (
	"fmt"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/ansi"
)

var _ tuicast.Terminal = (*Terminal)(nil)

// Terminal maintains the state of a VT220-compatible screen.
type Terminal struct {
	*ansi.Terminal
}

// New creates a VT220 terminal with the requested dimensions.
func New(width, height int) (*Terminal, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("creating VT220 terminal: dimensions must be positive")
	}
	terminal, err := ansi.New(width, height, ansi.VT220)
	if err != nil {
		return nil, fmt.Errorf("creating VT220 terminal: %w", err)
	}
	return &Terminal{Terminal: terminal}, nil
}
