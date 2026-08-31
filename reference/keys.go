package reference

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const keyHistoryLimit = 8

type keyModel struct {
	width   int
	height  int
	last    tea.KeyPressMsg
	hasLast bool
	paste   string
	count   int
	history []string
}

func newKeyModel(width, height int) keyModel {
	return keyModel{width: max(1, width), height: max(1, height)}
}

func (m *keyModel) update(key tea.KeyPressMsg) bool {
	if key.Code == tea.KeyEsc && key.Mod == 0 {
		return true
	}
	m.last, m.hasLast = key, true
	m.count++
	description := describeKey(key)
	m.history = append(m.history, description)
	if len(m.history) > keyHistoryLimit {
		m.history = append([]string(nil), m.history[len(m.history)-keyHistoryLimit:]...)
	}
	return false
}

func (m *keyModel) updatePaste(message tea.PasteMsg) {
	m.hasLast = false
	m.paste = message.Content
	m.count++
	m.history = append(m.history, "paste "+strconv.Quote(message.Content))
	if len(m.history) > keyHistoryLimit {
		m.history = append([]string(nil), m.history[len(m.history)-keyHistoryLimit:]...)
	}
}

func (m *keyModel) resize(width, height int) {
	m.width = max(1, width)
	m.height = max(1, height)
}

func (m keyModel) view() string {
	lines := []string{headingStyle.Render("FUNCTION KEYS"), "", fmt.Sprintf("Captured count: %d", m.count)}
	if m.hasLast {
		lines = append(lines,
			"Last: "+describeKey(m.last),
			fmt.Sprintf("Code: %d (%s)", m.last.Code, tea.Key(m.last).Keystroke()),
			fmt.Sprintf("Text: %s", strconv.Quote(m.last.Text)),
			"Modifiers: "+keyModifiers(m.last),
		)
	} else if m.paste != "" {
		lines = append(lines,
			"Last: paste "+strconv.Quote(m.paste),
			"Modifiers: none",
		)
	} else {
		lines = append(lines, "Last: none", "Modifiers: none")
	}
	lines = append(lines,
		"",
		"Exercise F1-F12 and Shift/Control/Alt/Meta combinations.",
		"Legacy Shift-F1..F12 may arrive as F13..F24.",
		"",
		"Recent (oldest to newest):",
	)
	if len(m.history) == 0 {
		lines = append(lines, "  none")
	} else {
		for index, entry := range m.history {
			lines = append(lines, fmt.Sprintf("  %d. %s", m.count-len(m.history)+index+1, entry))
		}
	}
	lines = append(lines, "", helpStyle.Render("Press keys to inspect them (including Ctrl-C)  Esc Return"))
	return strings.Join(lines, "\n")
}

func describeKey(key tea.KeyPressMsg) string {
	name := key.String()
	if key.Code >= tea.KeyF13 && key.Code <= tea.KeyF24 {
		name = fmt.Sprintf("f%d (legacy shift+f%d convention)", int(key.Code-tea.KeyF1)+1, int(key.Code-tea.KeyF13)+1)
	} else if name == "" {
		name = fmt.Sprintf("unknown key code %d", key.Code)
	}
	if key.IsRepeat {
		name += " [repeat]"
	}
	return name
}

func keyModifiers(key tea.KeyPressMsg) string {
	modifiers := make([]string, 0, 4)
	if key.Mod&tea.ModCtrl != 0 {
		modifiers = append(modifiers, "control")
	}
	if key.Mod&tea.ModAlt != 0 {
		modifiers = append(modifiers, "alt")
	}
	if key.Mod&tea.ModShift != 0 || key.Code >= tea.KeyF13 && key.Code <= tea.KeyF24 {
		modifiers = append(modifiers, "shift")
	}
	if key.Mod&tea.ModMeta != 0 {
		modifiers = append(modifiers, "meta")
	}
	if len(modifiers) == 0 {
		return "none"
	}
	return strings.Join(modifiers, ", ")
}
