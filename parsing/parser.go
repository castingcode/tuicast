// Package parsing converts a terminal byte stream into ANSI/DEC parser events.
package parsing

import (
	"bytes"

	"github.com/charmbracelet/x/ansi"
	ansiparser "github.com/charmbracelet/x/ansi/parser"
)

// Command identifies an ANSI escape command.
type Command struct {
	Prefix       byte
	Intermediate byte
	Final        byte
}

// Parameter is one CSI parameter. Present distinguishes an omitted parameter
// from an explicit zero. More reports that a subparameter follows.
type Parameter struct {
	Value   int
	Present bool
	More    bool
}

// Handler receives parser events synchronously during Parser.Write. Control
// string data is valid only for the duration of the call.
type Handler struct {
	Print   func(rune)
	Execute func(byte)
	CSI     func(Command, []Parameter)
	Escape  func(Command)
	// OSC receives an operating system command. Command is its leading
	// numeric identifier, or -1 when it has none; data follows the separating
	// semicolon.
	OSC func(command int, data []byte)
	// DCS receives a device control string's command, parameters, and data.
	DCS func(Command, []Parameter, []byte)
	// ControlString reports a PM, APC, or SOS string; kind is "PM", "APC",
	// or "SOS". Their content is not delivered.
	ControlString func(kind string)
}

// Parser is an incremental ANSI/DEC-compatible byte-stream parser.
type Parser struct {
	parser *ansi.Parser
	// continuation counts the UTF-8 continuation bytes still expected inside
	// a control string.
	continuation int
}

// stringPlaceholder stands in for a UTF-8 continuation byte of 0x9C inside an
// OSC or DCS string. It never occurs in valid UTF-8.
const stringPlaceholder = 0xFE

// New creates a parser that dispatches events to handler.
func New(handler Handler) *Parser {
	parser := ansi.NewParser()
	controlString := func(kind string) func([]byte) {
		return func([]byte) {
			if handler.ControlString != nil {
				handler.ControlString(kind)
			}
		}
	}
	parser.SetHandler(ansi.Handler{
		Print:   handler.Print,
		Execute: handler.Execute,
		HandleCsi: func(command ansi.Cmd, parameters ansi.Params) {
			if handler.CSI != nil {
				handler.CSI(convertCommand(command), convertParameters(parameters))
			}
		},
		HandleEsc: func(command ansi.Cmd) {
			if handler.Escape != nil {
				handler.Escape(convertCommand(command))
			}
		},
		HandleOsc: func(command int, data []byte) {
			if handler.OSC == nil {
				return
			}
			data = restorePlaceholders(data)
			if command == ansiparser.MissingCommand {
				handler.OSC(-1, data)
				return
			}
			_, payload, _ := bytes.Cut(data, []byte{';'})
			handler.OSC(command, payload)
		},
		HandleDcs: func(command ansi.Cmd, parameters ansi.Params, data []byte) {
			if handler.DCS != nil {
				handler.DCS(convertCommand(command), convertParameters(parameters), restorePlaceholders(data))
			}
		},
		HandlePm:  controlString("PM"),
		HandleApc: controlString("APC"),
		HandleSos: controlString("SOS"),
	})

	return &Parser{parser: parser}
}

// Write incrementally parses data. Incomplete sequences remain buffered until
// a later call supplies the remaining bytes.
func (p *Parser) Write(data []byte) (int, error) {
	for _, b := range data {
		p.parser.Advance(p.protectString(b))
	}
	return len(data), nil
}

// protectString keeps UTF-8 text inside control strings from ending them
// early. The underlying parser decodes UTF-8 only outside control strings, so
// it reads the continuation byte 0x9C as an 8-bit string terminator, and it
// ends PM, APC, and SOS strings at any non-ASCII byte. The rest of the string
// would then be printed on the screen.
func (p *Parser) protectString(b byte) byte {
	state := p.parser.State()
	switch state {
	case ansiparser.OscStringState, ansiparser.DcsStringState,
		ansiparser.SosStringState, ansiparser.PmStringState, ansiparser.ApcStringState:
	default:
		p.continuation = 0
		return b
	}
	switch {
	case p.continuation > 0 && b >= 0x80 && b <= 0xBF:
		p.continuation--
	case b >= 0xC2 && b <= 0xDF:
		p.continuation = 1
	case b >= 0xE0 && b <= 0xEF:
		p.continuation = 2
	case b >= 0xF0 && b <= 0xF4:
		p.continuation = 3
	default:
		p.continuation = 0
		return b
	}
	if state == ansiparser.OscStringState || state == ansiparser.DcsStringState {
		if b == 0x9C {
			return stringPlaceholder
		}
		return b
	}
	// PM, APC, and SOS content is not delivered, so any byte they accept
	// will do.
	return '?'
}

func restorePlaceholders(data []byte) []byte {
	if bytes.IndexByte(data, stringPlaceholder) < 0 {
		return data
	}
	return bytes.ReplaceAll(data, []byte{stringPlaceholder}, []byte{0x9C})
}

func convertCommand(command ansi.Cmd) Command {
	return Command{
		Prefix:       command.Prefix(),
		Intermediate: command.Intermediate(),
		Final:        command.Final(),
	}
}

func convertParameters(parameters ansi.Params) []Parameter {
	converted := make([]Parameter, len(parameters))
	for i := range parameters {
		value, more, _ := parameters.Param(i, 0)
		present := int(parameters[i])&ansiparser.ParamMask != ansiparser.MissingParam
		converted[i] = Parameter{Value: value, Present: present, More: more}
	}
	return converted
}
