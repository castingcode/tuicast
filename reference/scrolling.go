package reference

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const initialEventCount = 100

type scrollingModel struct {
	width      int
	height     int
	events     []string
	cursor     int
	follow     bool
	streaming  bool
	remaining  int
	generation uint64
	status     string
}

type scrollTickMsg struct {
	generation uint64
}

func newScrollingModel(width, height int) scrollingModel {
	m := scrollingModel{width: width, height: height}
	m.reset()
	return m
}

func (m *scrollingModel) update(message tea.Msg) (bool, tea.Cmd) {
	if tick, ok := message.(scrollTickMsg); ok {
		if !m.streaming || tick.generation != m.generation {
			return false, nil
		}
		m.appendEvent()
		m.remaining--
		if m.remaining == 0 {
			m.streaming = false
			m.status = "STREAM COMPLETE (5 EVENTS)"
			return false, nil
		}
		return false, scrollingTick(m.generation)
	}

	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}
	switch key.String() {
	case "esc", "f2":
		m.streaming = false
		m.generation++
		return true, nil
	case "up":
		m.move(-1)
	case "down":
		m.move(1)
	case "pgup":
		m.move(-m.pageSize())
	case "pgdown":
		m.move(m.pageSize())
	case "home":
		m.cursor = 0
		m.follow = false
	case "end":
		m.cursor = len(m.events) - 1
	case "f", "F":
		m.follow = !m.follow
		if m.follow {
			m.cursor = len(m.events) - 1
		}
		m.status = fmt.Sprintf("FOLLOW %s", onOff(m.follow))
	case "a", "A":
		m.appendEvent()
		m.status = "APPENDED 1 EVENT"
	case "r", "R":
		m.reset()
		m.status = "RESET TO 100 EVENTS"
	case "s", "S":
		m.streaming = true
		m.remaining = 5
		m.generation++
		m.status = "STREAMING"
		return false, scrollingTick(m.generation)
	}
	return false, nil
}

func (m *scrollingModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.clampCursor()
}

func (m scrollingModel) view() string {
	lines := []string{
		headingStyle.Render("DETERMINISTIC EVENT LOG"),
		fmt.Sprintf("EVENT   SOURCE    LEVEL  MESSAGE                         [%d total]", len(m.events)),
	}

	pageSize := m.pageSize()
	start := m.cursor - pageSize + 1
	if start < 0 {
		start = 0
	}
	if maximum := len(m.events) - pageSize; start > maximum && maximum >= 0 {
		start = maximum
	}
	end := min(len(m.events), start+pageSize)
	for index := start; index < end; index++ {
		row := m.events[index]
		if index == m.cursor {
			row = selectedStyle.Render(row)
		}
		lines = append(lines, row)
	}

	state := fmt.Sprintf("Row %d/%d  Follow: %s", m.cursor+1, len(m.events), onOff(m.follow))
	if m.streaming {
		state += "  STREAMING"
	}
	lines = append(lines, state)
	if m.status != "" {
		lines = append(lines, successStyle.Render(m.status))
	}
	lines = append(lines, helpStyle.Render("Up/Down PgUp/PgDn Home/End Navigate  F Follow  A Append  S Stream  R Reset  Esc/F2 Menu"))
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}

func (m *scrollingModel) reset() {
	m.events = make([]string, initialEventCount)
	for index := range m.events {
		m.events[index] = eventRow(index + 1)
	}
	m.cursor = 0
	m.follow = false
	m.streaming = false
	m.remaining = 0
	m.generation++
	m.status = ""
}

func (m *scrollingModel) appendEvent() {
	m.events = append(m.events, eventRow(len(m.events)+1))
	if m.follow {
		m.cursor = len(m.events) - 1
	}
}

func (m *scrollingModel) move(delta int) {
	m.cursor += delta
	m.clampCursor()
	if m.cursor != len(m.events)-1 {
		m.follow = false
	}
}

func (m *scrollingModel) clampCursor() {
	m.cursor = max(0, min(m.cursor, len(m.events)-1))
}

func (m scrollingModel) pageSize() int {
	return max(1, m.height-8)
}

func eventRow(number int) string {
	sources := [...]string{"PICKER", "PACKER", "SORTER", "GATEWAY"}
	levels := [...]string{"INFO", "WARN", "DEBUG", "INFO"}
	return fmt.Sprintf("%05d   %-9s %-6s Processed deterministic event %03d", number, sources[(number-1)%len(sources)], levels[(number-1)%len(levels)], number)
}

func onOff(value bool) string {
	if value {
		return "ON"
	}
	return "OFF"
}

func scrollingTick(generation uint64) tea.Cmd {
	return tea.Tick(75*time.Millisecond, func(time.Time) tea.Msg {
		return scrollTickMsg{generation: generation}
	})
}
