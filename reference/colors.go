package reference

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type colorsModel struct {
	width  int
	height int
}

func newColorsModel(width, height int) colorsModel {
	return colorsModel{width: width, height: height}
}

func (m *colorsModel) update(key tea.KeyPressMsg) bool {
	return key.String() == "esc" || key.String() == "f2"
}

func (m *colorsModel) resize(width, height int) {
	m.width = width
	m.height = height
}

func (m colorsModel) view() string {
	lines := []string{
		headingStyle.Render("ANSI COLORS AND ATTRIBUTES"),
		"",
		"ANSI 8 COLORS (foreground)",
		colorSwatches(30, 8, 0),
		"ANSI 8 COLORS (background)",
		backgroundSwatches(40, 8, 0),
		"ANSI 16 COLORS (bright foreground)",
		colorSwatches(90, 8, 8),
		"ANSI 16 COLORS (bright background)",
		backgroundSwatches(100, 8, 8),
		"",
		"XTERM 256 SELECTED COLOR CUBE SAMPLES",
		xtermSwatches([]int{16, 21, 46, 51, 196, 201, 226, 231}),
		"XTERM 256 SELECTED GRAYSCALE SAMPLES",
		xtermSwatches([]int{232, 236, 240, 244, 248, 252, 255}),
		"",
		"DEFAULT RESET: " + ansi("31", "RED") + " \x1b[39mDEFAULT\x1b[0m",
		"ATTRIBUTES: " + strings.Join([]string{
			ansi("1", "BOLD"),
			ansi("4", "UNDERLINE"),
			ansi("5", "BLINK"),
			ansi("7", "REVERSE"),
			"CONCEAL " + ansi("8", "HIDDEN") + " END",
		}, "  "),
		"",
		helpStyle.Render("Esc/F2 Menu"),
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}

func colorSwatches(firstCode, count, labelOffset int) string {
	parts := make([]string, count)
	for index := range parts {
		parts[index] = fmt.Sprintf("\x1b[%dm %02d \x1b[0m", firstCode+index, labelOffset+index)
	}
	return strings.Join(parts, " ")
}

func backgroundSwatches(firstCode, count, labelOffset int) string {
	parts := make([]string, count)
	for index := range parts {
		foreground := 30
		if index == 0 || index == 4 {
			foreground = 97
		}
		parts[index] = fmt.Sprintf("\x1b[%d;%dm %02d \x1b[0m", foreground, firstCode+index, labelOffset+index)
	}
	return strings.Join(parts, " ")
}

func xtermSwatches(colors []int) string {
	parts := make([]string, len(colors))
	for index, color := range colors {
		parts[index] = fmt.Sprintf("\x1b[48;5;%dm\x1b[38;5;%dm %03d \x1b[0m", color, contrastingColor(color), color)
	}
	return strings.Join(parts, " ")
}

func contrastingColor(color int) int {
	if color == 16 || color == 21 || color == 196 || color < 244 {
		return 15
	}
	return 0
}

func ansi(code, text string) string {
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}
