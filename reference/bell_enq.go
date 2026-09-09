package reference

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	bellByte = "\x07"
	enqByte  = "\x05"
)

type bellENQModel struct {
	width      int
	height     int
	bellCount  int
	enqCount   int
	answerback string
	waiting    bool
	events     []string
}

func newBellENQModel(width, height int) bellENQModel {
	return bellENQModel{width: width, height: height}
}

func (m *bellENQModel) update(key tea.KeyPressMsg) (bool, tea.Cmd) {
	// Once ENQ has been sent, printable terminal input is answerback data. Do
	// this before interpreting shortcuts so answerbacks containing B or E are
	// not mistaken for another request.
	if m.waiting && key.Text != "" && !key.IsRepeat {
		m.answerback += key.Text
		m.events = append(m.events, fmt.Sprintf("%d. Answerback received: %s", len(m.events)+1, strconv.Quote(key.Text)))
		return false, nil
	}
	switch key.String() {
	case "esc", "f2":
		return true, nil
	case "b", "B":
		if m.bellCount == 0 {
			m.bellCount = 1
			m.events = append(m.events, fmt.Sprintf("%d. BEL emitted", len(m.events)+1))
			return false, tea.Raw(bellByte)
		}
		return false, nil
	case "e", "E":
		if m.bellCount == 1 && m.enqCount == 0 {
			m.enqCount = 1
			m.waiting = true
			m.events = append(m.events, fmt.Sprintf("%d. ENQ emitted", len(m.events)+1))
			return false, tea.Raw(enqByte)
		}
		return false, nil
	}

	return false, nil
}

func (m *bellENQModel) resize(width, height int) {
	m.width = width
	m.height = height
}

func (m bellENQModel) view() string {
	status := "NOT REQUESTED"
	if m.waiting {
		status = "WAITING FOR ANSWERBACK"
	}
	if m.answerback != "" {
		status = "ANSWERBACK RECEIVED"
	}
	answerback := "none"
	if m.answerback != "" {
		answerback = strconv.Quote(m.answerback)
	}
	lines := []string{
		headingStyle.Render("BELL / ENQ ANSWERBACK"),
		"",
		fmt.Sprintf("BEL emitted: %d / 1", m.bellCount),
		fmt.Sprintf("ENQ emitted: %d / 1", m.enqCount),
		"Status: " + status,
		"Answerback: " + answerback,
		"",
		"Event order:",
	}
	if len(m.events) == 0 {
		lines = append(lines, "  none")
	} else {
		lines = append(lines, m.events...)
	}
	lines = append(lines, "", helpStyle.Render("B Emit BEL once  E Emit ENQ once (after BEL)  Esc/F2 Menu"))
	return strings.Join(lines, "\n")
}
