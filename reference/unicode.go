package reference

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type unicodeSample struct {
	label string
	text  string
	width int
}

var unicodeSamples = []unicodeSample{
	{label: "ASCII", text: "Hello", width: 5},
	{label: "Combining accent", text: "e\u0301", width: 1},
	{label: "CJK", text: "漢字", width: 4},
	{label: "Emoji", text: "🙂", width: 2},
	{label: "ZWJ", text: "👩‍💻", width: 2},
	{label: "Flag", text: "🇺🇸", width: 2},
	{label: "Skin tone", text: "👍🏽", width: 2},
}

type unicodeModel struct {
	width   int
	height  int
	input   textinput.Model
	focused bool
}

func newUnicodeModel(width, height int) unicodeModel {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "edit Unicode graphemes here"
	input.CharLimit = 64
	input.Focus()
	model := unicodeModel{width: width, height: height, input: input, focused: true}
	model.setInputWidth()
	return model
}

func (m *unicodeModel) update(key tea.KeyPressMsg) bool {
	switch key.String() {
	case "esc", "f2":
		return true
	case "tab":
		m.focused = !m.focused
		if m.focused {
			m.input.Focus()
		} else {
			m.input.Blur()
		}
		return false
	}
	if m.focused {
		m.input, _ = m.input.Update(key)
	}
	return false
}

func (m *unicodeModel) updatePaste(message tea.PasteMsg) tea.Cmd {
	if !m.focused {
		return nil
	}
	var command tea.Cmd
	m.input, command = m.input.Update(message)
	return command
}

func (m *unicodeModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.setInputWidth()
}

func (m *unicodeModel) setInputWidth() {
	m.input.SetWidth(min(48, max(8, m.width-4)))
}

func (m unicodeModel) view() string {
	lines := []string{
		headingStyle.Render("UNICODE ALIGNMENT"),
		"",
		"Sample              |Text      | Expected width",
	}
	for _, sample := range unicodeSamples {
		cell := lipgloss.NewStyle().Width(10).Render(sample.text)
		lines = append(lines, fmt.Sprintf("%-19s |%s| %d", sample.label, cell, sample.width))
	}
	focus := "focused"
	if !m.focused {
		focus = "unfocused"
	}
	lines = append(lines,
		"",
		"Editable grapheme input ("+focus+"):",
		m.input.View(),
		"Value is retained across resize events.",
		"",
		helpStyle.Render("Tab Toggle input focus  Esc/F2 Menu"),
	)
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}
