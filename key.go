package tuicast

import "fmt"

// Key identifies a named terminal key or one printable character.
type Key string

// KeyModifier identifies modifiers applied to a key press. Alt and Meta are
// aliases because ANSI terminals conventionally encode both as an Escape
// prefix or the same CSI modifier bit.
type KeyModifier uint8

const (
	ModifierShift KeyModifier = 1 << iota
	ModifierAlt
	ModifierControl

	ModifierMeta = ModifierAlt
)

// KeyPress describes one key and its combined modifiers.
type KeyPress struct {
	Key       Key
	Modifiers KeyModifier
}

// NewKeyPress combines modifiers into one key press.
func NewKeyPress(key Key, modifiers ...KeyModifier) (KeyPress, error) {
	var combined KeyModifier
	for _, modifier := range modifiers {
		if modifier == 0 || modifier&^(ModifierShift|ModifierAlt|ModifierControl) != 0 {
			return KeyPress{}, fmt.Errorf("creating key press: unknown modifier %d", modifier)
		}
		combined |= modifier
	}
	return KeyPress{Key: key, Modifiers: combined}, nil
}

const (
	KeyEnter     Key = "Enter"
	KeyTab       Key = "Tab"
	KeyBackspace Key = "Backspace"
	KeyEscape    Key = "Escape"
	KeyUp        Key = "ArrowUp"
	KeyDown      Key = "ArrowDown"
	KeyRight     Key = "ArrowRight"
	KeyLeft      Key = "ArrowLeft"
	KeyHome      Key = "Home"
	KeyEnd       Key = "End"
	KeyInsert    Key = "Insert"
	KeyDelete    Key = "Delete"
	KeyPageUp    Key = "PageUp"
	KeyPageDown  Key = "PageDown"
	KeyF1        Key = "F1"
	KeyF2        Key = "F2"
	KeyF3        Key = "F3"
	KeyF4        Key = "F4"
	KeyF5        Key = "F5"
	KeyF6        Key = "F6"
	KeyF7        Key = "F7"
	KeyF8        Key = "F8"
	KeyF9        Key = "F9"
	KeyF10       Key = "F10"
	KeyF11       Key = "F11"
	KeyF12       Key = "F12"
)

// KeyEncoder converts named keys to the byte sequence expected by a terminal
// profile. Terminal implementations may satisfy this in addition to Terminal.
type KeyEncoder interface {
	EncodeKey(Key) ([]byte, error)
}

// ModifiedKeyEncoder converts key presses with modifiers to the byte sequence
// expected by a terminal profile.
type ModifiedKeyEncoder interface {
	EncodeKeyPress(KeyPress) ([]byte, error)
}
