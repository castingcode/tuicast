package reference

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// cursorModel supplies an absolute cursor position with its diagnostic grid so
// the terminal cursor can be compared with the displayed expectation.
type cursorModel struct {
	width   int
	height  int
	column  int
	row     int
	visible bool
	saved   bool
	savedX  int
	savedY  int
}

func newCursorModel(width, height int) cursorModel {
	m := cursorModel{visible: true}
	m.resize(width, height)
	m.center()
	return m
}

func (m *cursorModel) update(key tea.KeyPressMsg) (bool, tea.Cmd) {
	switch key.Code {
	case tea.KeyEsc:
		return true, nil
	case tea.KeyLeft:
		m.column--
	case tea.KeyRight:
		m.column++
	case tea.KeyUp:
		m.row--
	case tea.KeyDown:
		m.row++
	case tea.KeyHome:
		m.center()
	case tea.KeyF1:
		m.savedX, m.savedY, m.saved = m.column, m.row, true
	case tea.KeyF2:
		if m.saved {
			m.column, m.row = m.savedX, m.savedY
		}
	case tea.KeyF3:
		m.visible = !m.visible
	case tea.KeyF4:
		// A fixed sequence (right, down, down, left) has a predictable net
		// result while still exercising clamping at every step.
		m.move(1, 0)
		m.move(0, 1)
		m.move(0, 1)
		m.move(-1, 0)
	}
	m.clamp()
	return false, nil
}

func (m *cursorModel) resize(width, height int) {
	m.width = max(1, width)
	m.height = max(1, height)
	m.clamp()
	if m.saved {
		m.savedX = min(max(0, m.savedX), m.width-1)
		m.savedY = min(max(0, m.savedY), m.height-1)
	}
}

func (m cursorModel) view() string {
	gridWidth := min(m.width, 40)
	gridHeight := min(m.height, 8)
	left := min(max(0, m.column-gridWidth/2), m.width-gridWidth)
	top := min(max(0, m.row-gridHeight/2), m.height-gridHeight)

	lines := []string{
		headingStyle.Render("CURSOR MOVEMENT"),
		"",
		fmt.Sprintf("Expected terminal cursor: column=%d row=%d visible=%t", m.column, m.row, m.visible),
		"Coordinates are zero-based and match session.screen.cursor.",
		fmt.Sprintf("Grid viewport: columns %d-%d, rows %d-%d", left, left+gridWidth-1, top, top+gridHeight-1),
	}
	for y := top; y < top+gridHeight; y++ {
		line := make([]rune, gridWidth)
		for x := range line {
			line[x] = '·'
		}
		if m.visible && y == m.row {
			line[m.column-left] = 'X'
		}
		lines = append(lines, fmt.Sprintf("%3d %s", y, string(line)))
	}
	lines = append(lines, "", helpStyle.Render("Arrows Move  Home Center  F1 Save  F2 Restore  F3 Show/Hide  F4 Script  Esc Return"))
	return strings.Join(lines, "\n")
}

func (m *cursorModel) center() {
	m.column = (m.width - 1) / 2
	m.row = (m.height - 1) / 2
}

func (m *cursorModel) move(dx, dy int) {
	m.column += dx
	m.row += dy
	m.clamp()
}

func (m *cursorModel) clamp() {
	m.column = min(max(0, m.column), max(0, m.width-1))
	m.row = min(max(0, m.row), max(0, m.height-1))
}
