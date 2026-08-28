package tuicast

import "strings"

// Color is a terminal palette index. DefaultColor selects the terminal's
// configured default foreground or background.
type Color int16

const (
	DefaultColor Color = -1
)

const (
	Black Color = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Attributes describes a cell's graphic rendition.
type Attributes uint16

const (
	Bold Attributes = 1 << iota
	Underline
	Blink
	Reverse
	Conceal
)

// Has reports whether all requested attributes are set.
func (a Attributes) Has(attributes Attributes) bool {
	return a&attributes == attributes
}

// Cell is one terminal screen cell. Text is normally one character. Width is
// one for a normal cell, two for a wide character, and zero for its continuation.
type Cell struct {
	Text       string
	Width      int
	Foreground Color
	Background Color
	Attributes Attributes
}

// Cursor is a zero-based cursor position and its visibility state.
type Cursor struct {
	Column  int
	Row     int
	Visible bool
}

// Screen is a detached, row-major terminal snapshot. The Cells slice belongs
// to the snapshot and is never reused by the terminal that produced it.
type Screen struct {
	Width    int
	Height   int
	Cells    []Cell
	Cursor   Cursor
	Revision uint64
}

// CellAt returns the cell at a zero-based position.
func (s Screen) CellAt(column, row int) (Cell, bool) {
	if column < 0 || column >= s.Width || row < 0 || row >= s.Height {
		return Cell{}, false
	}

	index := row*s.Width + column
	if index >= len(s.Cells) {
		return Cell{}, false
	}

	return s.Cells[index], true
}

// Line returns one row's text, including trailing spaces.
func (s Screen) Line(row int) string {
	if row < 0 || row >= s.Height {
		return ""
	}

	var line strings.Builder
	for column := 0; column < s.Width; column++ {
		cell, ok := s.CellAt(column, row)
		if !ok {
			break
		}
		if cell.Width != 0 {
			line.WriteString(cell.Text)
		}
	}
	return line.String()
}

// Text returns every row joined by a newline. It preserves trailing spaces and
// does not append a newline after the final row.
func (s Screen) Text() string {
	var text strings.Builder
	for row := 0; row < s.Height; row++ {
		if row > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(s.Line(row))
	}
	return text.String()
}
