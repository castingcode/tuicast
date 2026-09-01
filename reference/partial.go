package reference

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type partialMode uint8

const (
	partialNone partialMode = iota
	partialPrematureReady
	partialIndependentRegions
	partialProgress
)

type partialTickMsg struct {
	generation uint64
}

type partialModel struct {
	width      int
	height     int
	mode       partialMode
	step       int
	running    bool
	generation uint64
	ignored    int
	accepted   string
}

func newPartialModel(width, height int) partialModel {
	return partialModel{width: width, height: height}
}

func (m *partialModel) update(message tea.Msg) (bool, tea.Cmd) {
	if tick, ok := message.(partialTickMsg); ok {
		if !m.running || tick.generation != m.generation {
			return false, nil
		}
		m.step++
		if m.step >= m.lastStep() {
			m.running = false
			return false, nil
		}
		return false, partialTick(m.generation)
	}

	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}
	if key.String() == "esc" || key.String() == "f2" {
		m.running = false
		m.generation++
		return true, nil
	}
	switch key.String() {
	case "1":
		return false, m.start(partialPrematureReady)
	case "2":
		return false, m.start(partialIndependentRegions)
	case "3":
		return false, m.start(partialProgress)
	case "r", "R":
		if m.mode != partialNone {
			return false, m.start(m.mode)
		}
	}

	if m.mode == partialPrematureReady && key.Text != "" {
		if m.running || m.step < m.lastStep() {
			m.ignored++
		} else {
			m.accepted += key.Text
		}
	}
	return false, nil
}

func (m *partialModel) start(mode partialMode) tea.Cmd {
	m.mode = mode
	m.step = 0
	m.running = true
	m.generation++
	m.ignored = 0
	m.accepted = ""
	return partialTick(m.generation)
}

func (m *partialModel) resize(width, height int) {
	m.width = width
	m.height = height
}

func (m partialModel) view() string {
	lines := []string{headingStyle.Render("PARTIAL SCREEN UPDATES"), ""}
	if m.mode == partialNone {
		lines = append(lines,
			"1. Premature READY and gated input",
			"2. Independently updating screen regions",
			"3. In-place spinner and progress bar",
			"",
			"Each scenario uses deterministic timed updates.",
		)
	} else {
		switch m.mode {
		case partialPrematureReady:
			lines = append(lines, m.prematureReadyView()...)
		case partialIndependentRegions:
			lines = append(lines, m.independentRegionsView()...)
		case partialProgress:
			lines = append(lines, m.progressView()...)
		}
		lines = append(lines, "", "R Restart this scenario")
	}
	lines = append(lines, "", helpStyle.Render("1/2/3 Start scenario  R Restart  Esc/F2 Menu"))
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}

func (m partialModel) prematureReadyView() []string {
	lines := []string{"PREMATURE READY / INPUT GATING", ""}
	if m.step == 0 {
		return append(lines, "INITIALIZING")
	}
	lines = append(lines, successStyle.Render("READY"), "")
	if m.step >= 2 {
		lines = append(lines, "Order: ORD-10002342")
	}
	if m.step >= 3 {
		lines = append(lines, "Zone: PICK-FACE-07")
	}
	if m.step >= 4 {
		lines = append(lines,
			"Inventory: VERIFIED",
			"",
			successStyle.Render("SCREEN COMPLETE / INPUT ENABLED"),
			fmt.Sprintf("Ignored early key events: %d", m.ignored),
			"Accepted input: "+m.accepted,
		)
	} else {
		lines = append(lines, "Input remains disabled while the screen is rendering.")
	}
	return lines
}

func (m partialModel) independentRegionsView() []string {
	header, order, counters, footer := "STARTING", "pending", "0 / 0", "waiting"
	if m.step >= 1 {
		footer = "connection active"
	}
	if m.step >= 2 {
		order = "ORD-10002342 / PICKING"
	}
	if m.step >= 3 {
		header = "WAVE-17 ACTIVE"
	}
	if m.step >= 4 {
		counters = "18 processed / 2 remaining"
	}
	if m.step >= 5 {
		order = "ORD-10002342 / SHIPPED"
		counters = "20 processed / 0 remaining"
		footer = "REGIONS COMPLETE"
	}
	return []string{
		"INDEPENDENT REGIONS",
		"",
		"Header:   " + header,
		"Order:    " + order,
		"Counters: " + counters,
		"Footer:   " + footer,
	}
}

func (m partialModel) progressView() []string {
	spinners := [...]string{"|", "/", "-", "\\"}
	percent := min(100, m.step*10)
	filled := percent / 10
	status := "RUNNING"
	if percent == 100 {
		status = "COMPLETE"
	}
	return []string{
		"IN-PLACE PROGRESS",
		"",
		fmt.Sprintf("Spinner: %s", spinners[m.step%len(spinners)]),
		fmt.Sprintf("Progress: [%-10s] %3d%%", strings.Repeat("#", filled), percent),
		"Status: " + status,
	}
}

func (m partialModel) lastStep() int {
	switch m.mode {
	case partialPrematureReady:
		return 4
	case partialIndependentRegions:
		return 5
	case partialProgress:
		return 10
	default:
		return 0
	}
}

func partialTick(generation uint64) tea.Cmd {
	return tea.Tick(75*time.Millisecond, func(time.Time) tea.Msg {
		return partialTickMsg{generation: generation}
	})
}
