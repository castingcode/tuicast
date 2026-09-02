package tuicast

import "strings"

// Color is a terminal palette index. Minus one selects the terminal default.
type Color int16

const DefaultColor Color = -1

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

// Attributes is the terminal cell attribute bitmask.
type Attributes uint16

const (
	Bold Attributes = 1 << iota
	Underline
	Blink
	Reverse
	Conceal
)

// Has reports whether all requested attributes are set.
func (attributes Attributes) Has(requested Attributes) bool {
	return attributes&requested == requested
}

// Cell is one terminal screen cell.
type Cell struct {
	Text       string     `json:"text"`
	Width      int        `json:"width"`
	Foreground Color      `json:"foreground"`
	Background Color      `json:"background"`
	Attributes Attributes `json:"attributes"`
}

// Cursor is a zero-based terminal cursor position.
type Cursor struct {
	Column  int  `json:"column"`
	Row     int  `json:"row"`
	Visible bool `json:"visible"`
}

// Position is a zero-based position on a screen.
type Position struct {
	Column int
	Row    int
}

// Screen is a detached terminal snapshot returned by the driver.
type Screen struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Cells    []Cell `json:"cells"`
	Cursor   Cursor `json:"cursor"`
	Revision uint64 `json:"revision"`
	Content  string `json:"text"`
}

// Text returns the complete screen text, preserving rows and trailing spaces.
func (s Screen) Text() string { return s.Content }

// Contains reports whether exact text appears in the complete screen text.
func (s Screen) Contains(text string) bool { return strings.Contains(s.Content, text) }

// Find returns the first row-major position at which exact text begins. Text
// must begin and end within one row.
func (s Screen) Find(text string) (Position, bool) {
	if text == "" && s.Width > 0 && s.Height > 0 {
		return Position{}, true
	}
	for row := 0; row < s.Height; row++ {
		for column := 0; column < s.Width; column++ {
			cell, ok := s.CellAt(column, row)
			if !ok || cell.Width == 0 {
				continue
			}
			var candidate strings.Builder
			for current := column; current < s.Width; current++ {
				next, exists := s.CellAt(current, row)
				if !exists {
					break
				}
				if next.Width != 0 {
					candidate.WriteString(next.Text)
				}
				if strings.HasPrefix(candidate.String(), text) {
					return Position{Column: column, Row: row}, true
				}
				if !strings.HasPrefix(text, candidate.String()) {
					break
				}
			}
		}
	}
	return Position{}, false
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

// CellAt returns a cell at a zero-based position.
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
