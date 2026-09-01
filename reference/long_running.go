package reference

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type longRunningMode uint8

const (
	longRunningNone longRunningMode = iota
	longRunningFinite
	longRunningContinuous
	longRunningFailure
)

type longRunningTickMsg struct {
	generation uint64
}

type longRunningModel struct {
	width      int
	height     int
	mode       longRunningMode
	running    bool
	paused     bool
	generation uint64
	heartbeats int
	processed  int
	succeeded  int
	failed     int
	remaining  int
	status     string
	events     []string
}

func newLongRunningModel(width, height int) longRunningModel {
	return longRunningModel{width: width, height: height}
}

func (m *longRunningModel) update(message tea.Msg) (bool, tea.Cmd) {
	if tick, ok := message.(longRunningTickMsg); ok {
		if !m.running || m.paused || tick.generation != m.generation {
			return false, nil
		}
		m.advance()
		if !m.running {
			return false, nil
		}
		return false, longRunningTick(m.generation)
	}

	key, ok := message.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}
	if key.String() == "esc" || key.String() == "f2" {
		m.stop("OPERATION CANCELED")
		return true, nil
	}
	switch key.String() {
	case "1":
		return false, m.start(longRunningFinite)
	case "2":
		return false, m.start(longRunningContinuous)
	case "3":
		return false, m.start(longRunningFailure)
	case "p", "P":
		if m.running {
			m.paused = !m.paused
			m.generation++
			if m.paused {
				m.status = "OPERATION PAUSED"
				return false, nil
			}
			m.status = "OPERATION RUNNING"
			return false, longRunningTick(m.generation)
		}
	case "c", "C":
		if m.running {
			m.stop("OPERATION CANCELED")
		}
	case "s", "S":
		if m.running && m.mode == longRunningContinuous {
			m.stop("SESSION STOPPED")
		}
	case "r", "R":
		if m.mode != longRunningNone {
			return false, m.start(m.mode)
		}
	}
	return false, nil
}

func (m *longRunningModel) start(mode longRunningMode) tea.Cmd {
	m.mode = mode
	m.running = true
	m.paused = false
	m.generation++
	m.heartbeats = 0
	m.processed = 0
	m.succeeded = 0
	m.failed = 0
	m.events = nil
	m.status = "OPERATION RUNNING"
	switch mode {
	case longRunningFinite:
		m.remaining = 120
	case longRunningContinuous:
		m.remaining = -1
	case longRunningFailure:
		m.remaining = 100
	}
	return longRunningTick(m.generation)
}

func (m *longRunningModel) advance() {
	m.heartbeats++
	switch m.mode {
	case longRunningFinite:
		batch := min(3, m.remaining)
		m.processed += batch
		m.remaining -= batch
		m.failed = m.processed / 30
		m.succeeded = m.processed - m.failed
		if m.heartbeats%4 == 0 {
			m.appendEvent(fmt.Sprintf("Heartbeat %03d: processed %d of 120", m.heartbeats, m.processed))
		}
		if m.remaining == 0 {
			m.stop("OPERATION COMPLETE")
		}
	case longRunningContinuous:
		m.processed++
		m.succeeded = m.processed
		if m.heartbeats%4 == 0 {
			m.appendEvent(fmt.Sprintf("Heartbeat %03d: continuous session active", m.heartbeats))
		}
	case longRunningFailure:
		m.processed += 10
		m.remaining -= 10
		m.succeeded = m.processed
		if m.processed == 60 {
			m.failed = 1
			m.succeeded = m.processed - m.failed
			m.stop("OPERATION FAILED / E-WAVE-060")
		}
	}
}

func (m *longRunningModel) appendEvent(event string) {
	m.events = append(m.events, event)
	if len(m.events) > 8 {
		m.events = append([]string(nil), m.events[len(m.events)-8:]...)
	}
}

func (m *longRunningModel) stop(status string) {
	m.running = false
	m.paused = false
	m.generation++
	m.status = status
}

func (m *longRunningModel) resize(width, height int) {
	m.width = width
	m.height = height
}

func (m longRunningModel) view() string {
	lines := []string{headingStyle.Render("LONG-RUNNING OPERATION"), ""}
	if m.mode == longRunningNone {
		lines = append(lines,
			"1. Finite warehouse wave (approximately 10 seconds)",
			"2. Continuous bounded session / soak mode",
			"3. Deterministic failure at 60%",
		)
	} else {
		mode := map[longRunningMode]string{
			longRunningFinite:     "FINITE WAREHOUSE WAVE",
			longRunningContinuous: "CONTINUOUS SESSION",
			longRunningFailure:    "FAILURE SIMULATION",
		}[m.mode]
		remaining := fmt.Sprintf("%d", m.remaining)
		if m.remaining < 0 {
			remaining = "unbounded"
		}
		lines = append(lines,
			mode,
			"",
			fmt.Sprintf("Heartbeats: %d", m.heartbeats),
			fmt.Sprintf("Processed: %d", m.processed),
			fmt.Sprintf("Succeeded: %d", m.succeeded),
			fmt.Sprintf("Failed: %d", m.failed),
			"Remaining: "+remaining,
			"Status: "+m.status,
			"",
			"Recent events (bounded to 8):",
		)
		if len(m.events) == 0 {
			lines = append(lines, "  none")
		} else {
			for _, event := range m.events {
				lines = append(lines, "  "+event)
			}
		}
	}
	lines = append(lines, "", helpStyle.Render("1/2/3 Start  P Pause/Resume  C Cancel  S Stop continuous  R Retry  Esc/F2 Menu"))
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}

func longRunningTick(generation uint64) tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return longRunningTickMsg{generation: generation}
	})
}
