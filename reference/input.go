package reference

import "unicode/utf8"

type inputKind uint8

const (
	inputText inputKind = iota
	inputEnter
	inputTab
	inputBackspace
	inputUp
	inputDown
	inputLeft
	inputRight
	inputF1
	inputF2
	inputExit
)

type inputEvent struct {
	kind inputKind
	text rune
}

type inputDecoder struct {
	pending    []byte
	suppressLF bool
}

func (d *inputDecoder) feed(input []byte) []inputEvent {
	d.pending = append(d.pending, input...)
	var events []inputEvent
	for len(d.pending) > 0 {
		if d.pending[0] == '\x1b' {
			event, size, complete := decodeEscape(d.pending)
			if !complete {
				break
			}
			d.pending = d.pending[size:]
			if event.kind != inputText || event.text != 0 {
				events = append(events, event)
			}
			d.suppressLF = false
			continue
		}

		character := d.pending[0]
		switch character {
		case '\x03':
			events = append(events, inputEvent{kind: inputExit})
			d.pending = d.pending[1:]
			d.suppressLF = false
		case '\r':
			events = append(events, inputEvent{kind: inputEnter})
			d.pending = d.pending[1:]
			d.suppressLF = true
		case '\n':
			if !d.suppressLF {
				events = append(events, inputEvent{kind: inputEnter})
			}
			d.pending = d.pending[1:]
			d.suppressLF = false
		case '\t':
			events = append(events, inputEvent{kind: inputTab})
			d.pending = d.pending[1:]
			d.suppressLF = false
		case '\b', '\x7f':
			events = append(events, inputEvent{kind: inputBackspace})
			d.pending = d.pending[1:]
			d.suppressLF = false
		default:
			if !utf8.FullRune(d.pending) {
				return events
			}
			decoded, size := utf8.DecodeRune(d.pending)
			d.pending = d.pending[size:]
			d.suppressLF = false
			if decoded != utf8.RuneError || size > 1 {
				events = append(events, inputEvent{kind: inputText, text: decoded})
			}
		}
	}
	return events
}

func decodeEscape(input []byte) (inputEvent, int, bool) {
	if len(input) < 2 {
		return inputEvent{}, 0, false
	}
	if input[1] == 'O' {
		if len(input) < 3 {
			return inputEvent{}, 0, false
		}
		switch input[2] {
		case 'A':
			return inputEvent{kind: inputUp}, 3, true
		case 'B':
			return inputEvent{kind: inputDown}, 3, true
		case 'C':
			return inputEvent{kind: inputRight}, 3, true
		case 'D':
			return inputEvent{kind: inputLeft}, 3, true
		case 'P':
			return inputEvent{kind: inputF1}, 3, true
		case 'Q':
			return inputEvent{kind: inputF2}, 3, true
		default:
			return inputEvent{}, 3, true
		}
	}
	if input[1] != '[' {
		return inputEvent{}, 1, true
	}

	final := 2
	for final < len(input) && (input[final] < '@' || input[final] > '~') {
		final++
	}
	if final == len(input) {
		return inputEvent{}, 0, false
	}
	sequence := string(input[2 : final+1])
	switch sequence {
	case "A":
		return inputEvent{kind: inputUp}, final + 1, true
	case "B":
		return inputEvent{kind: inputDown}, final + 1, true
	case "C":
		return inputEvent{kind: inputRight}, final + 1, true
	case "D":
		return inputEvent{kind: inputLeft}, final + 1, true
	case "11~":
		return inputEvent{kind: inputF1}, final + 1, true
	case "12~":
		return inputEvent{kind: inputF2}, final + 1, true
	default:
		return inputEvent{}, final + 1, true
	}
}
