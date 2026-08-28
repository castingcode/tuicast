// Package parsing converts a terminal byte stream into ANSI/DEC parser events.
package parsing

import (
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

// Handler receives parser events synchronously during Parser.Write.
type Handler struct {
	Print   func(rune)
	Execute func(byte)
	CSI     func(Command, []Parameter)
	Escape  func(Command)
}

// Parser is an incremental ANSI/DEC-compatible byte-stream parser.
type Parser struct {
	parser *ansi.Parser
}

// New creates a parser that dispatches events to handler.
func New(handler Handler) *Parser {
	parser := ansi.NewParser()
	parser.SetHandler(ansi.Handler{
		Print:   handler.Print,
		Execute: handler.Execute,
		HandleCsi: func(command ansi.Cmd, parameters ansi.Params) {
			if handler.CSI == nil {
				return
			}

			converted := make([]Parameter, len(parameters))
			for i := range parameters {
				value, more, _ := parameters.Param(i, 0)
				present := int(parameters[i])&ansiparser.ParamMask != ansiparser.MissingParam
				converted[i] = Parameter{Value: value, Present: present, More: more}
			}
			handler.CSI(convertCommand(command), converted)
		},
		HandleEsc: func(command ansi.Cmd) {
			if handler.Escape != nil {
				handler.Escape(convertCommand(command))
			}
		},
	})

	return &Parser{parser: parser}
}

// Write incrementally parses data. Incomplete sequences remain buffered until
// a later call supplies the remaining bytes.
func (p *Parser) Write(data []byte) (int, error) {
	for _, b := range data {
		p.parser.Advance(b)
	}
	return len(data), nil
}

func convertCommand(command ansi.Cmd) Command {
	return Command{
		Prefix:       command.Prefix(),
		Intermediate: command.Intermediate(),
		Final:        command.Final(),
	}
}
