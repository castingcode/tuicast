package reference

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var resizeOrders = []string{"ORD-10002341", "ORD-10002342", "ORD-10002343"}

type resizeModel struct {
	width        int
	height       int
	targetWidth  int
	targetHeight int
	resizeCount  int
	selected     int
	input        textinput.Model
}

func newResizeModel(width, height int) resizeModel {
	input := textinput.New()
	input.Prompt = "Retained input: "
	input.Placeholder = "type before resizing"
	input.CharLimit = 32
	input.Focus()

	model := resizeModel{
		width:        width,
		height:       height,
		targetWidth:  80,
		targetHeight: 24,
		input:        input,
	}
	model.setInputWidth()
	return model
}

func (m *resizeModel) update(key tea.KeyPressMsg) bool {
	switch key.String() {
	case "esc", "f2":
		return true
	case "t", "T":
		if m.targetWidth == 80 {
			m.targetWidth = 132
		} else {
			m.targetWidth = 80
		}
		m.targetHeight = 24
		return false
	case "up":
		m.selected = (m.selected - 1 + len(resizeOrders)) % len(resizeOrders)
		return false
	case "down":
		m.selected = (m.selected + 1) % len(resizeOrders)
		return false
	}

	m.input, _ = m.input.Update(key)
	return false
}

func (m *resizeModel) updatePaste(message tea.PasteMsg) tea.Cmd {
	var command tea.Cmd
	m.input, command = m.input.Update(message)
	return command
}

func (m *resizeModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.resizeCount++
	m.setInputWidth()
}

func (m *resizeModel) setInputWidth() {
	m.input.SetWidth(min(32, max(8, m.width-18)))
}

func (m resizeModel) view() string {
	breakpoint := "Narrow"
	if m.width >= 100 {
		breakpoint = "Wide"
	} else if m.width >= 60 {
		breakpoint = "Medium"
	}
	status := errorStyle.Render("WAITING")
	if m.width == m.targetWidth && m.height == m.targetHeight {
		status = successStyle.Render("MATCH")
	}

	lines := []string{
		headingStyle.Render("TERMINAL RESIZE"),
		"",
		fmt.Sprintf("Actual dimensions: %dx%d", m.width, m.height),
		fmt.Sprintf("Target dimensions: %dx%d", m.targetWidth, m.targetHeight),
		fmt.Sprintf("Resize events: %d", m.resizeCount),
		"Breakpoint: " + breakpoint,
		"Status: " + status,
		"",
		"Retained state:",
	}
	for index, order := range resizeOrders {
		lines = append(lines, controlLine(index == m.selected, order))
	}
	lines = append(lines,
		m.input.View(),
		"",
		"This application cannot force a remote PTY resize.",
		"Automation: press T to toggle the target, then call driver session.resize.",
		"",
		helpStyle.Render("T Toggle  Up/Down Select  Type to edit  Esc/F2 Menu"),
	)
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}
