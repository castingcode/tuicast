package reference

import (
	"fmt"
	"strconv"
	"strings"

	bubbleskey "charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	formPurchaseOrder = iota
	formSKU
	formQuantity
	formBin
	formNotes
	formPriority
	formSubmit
	formCancel
	formControlCount
)

var formLabels = []string{"Purchase order", "SKU", "Quantity", "Bin location"}

type formModel struct {
	width     int
	height    int
	inputs    [4]textinput.Model
	notes     textarea.Model
	focus     int
	priority  int
	errors    [5]string
	result    string
	submitted bool
}

func newFormModel(width, height int) formModel {
	placeholders := []string{"PO-10002341", "WIDGET-42", "25", "A-01-02"}
	limits := []int{20, 24, 6, 12}
	var inputs [4]textinput.Model
	for index := range inputs {
		inputs[index] = textinput.New()
		inputs[index].Prompt = ""
		inputs[index].Placeholder = placeholders[index]
		inputs[index].CharLimit = limits[index]
	}
	inputs[formSKU].ShowSuggestions = true
	inputs[formSKU].SetSuggestions([]string{"WIDGET-42", "CAST-ROD-8", "REEL-2500", "LINE-6WT", "FLY-ADAMS", "PACK-DRY"})
	inputs[formSKU].KeyMap.AcceptSuggestion = bubbleskey.NewBinding(bubbleskey.WithKeys("ctrl+y"))
	inputs[formPurchaseOrder].Focus()
	notes := textarea.New()
	notes.Prompt = ""
	notes.Placeholder = "Optional receiving note"
	notes.ShowLineNumbers = false
	notes.CharLimit = 200
	notes.SetHeight(3)
	model := formModel{width: width, height: height, inputs: inputs, notes: notes}
	model.resize(width, height)
	return model
}

func (m *formModel) update(key tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.submitted {
		switch key.String() {
		case "enter", "esc", "f2":
			return true, nil
		case "r":
			*m = newFormModel(m.width, m.height)
			return false, textinput.Blink
		}
		return false, nil
	}

	switch key.String() {
	case "esc", "f2":
		return true, nil
	case "tab":
		return false, m.moveFocus(1)
	case "shift+tab":
		return false, m.moveFocus(-1)
	case "down":
		if m.focus == formNotes {
			return false, m.updateNotes(key)
		}
		return false, m.moveFocus(1)
	case "up":
		if m.focus == formNotes {
			return false, m.updateNotes(key)
		}
		return false, m.moveFocus(-1)
	case "enter":
		switch m.focus {
		case formNotes:
			return false, m.updateNotes(key)
		case formSubmit:
			m.submit()
			return false, nil
		case formCancel:
			return true, nil
		default:
			return false, m.moveFocus(1)
		}
	case "left", "right", " ":
		if m.focus == formPriority {
			m.priority = 1 - m.priority
			return false, nil
		}
	}

	if m.focus < len(m.inputs) {
		var command tea.Cmd
		m.inputs[m.focus], command = m.inputs[m.focus].Update(key)
		m.errors[m.focus] = ""
		m.result = ""
		return false, command
	}
	if m.focus == formNotes {
		return false, m.updateNotes(key)
	}
	return false, nil
}

func (m *formModel) updatePaste(message tea.PasteMsg) tea.Cmd {
	if m.submitted {
		return nil
	}
	if m.focus < len(m.inputs) {
		var command tea.Cmd
		m.inputs[m.focus], command = m.inputs[m.focus].Update(message)
		m.errors[m.focus] = ""
		m.result = ""
		return command
	}
	if m.focus == formNotes {
		var command tea.Cmd
		m.notes, command = m.notes.Update(message)
		m.errors[formNotes] = ""
		m.result = ""
		return command
	}
	return nil
}

func (m *formModel) updateNotes(key tea.KeyPressMsg) tea.Cmd {
	var command tea.Cmd
	m.notes, command = m.notes.Update(key)
	m.errors[formNotes] = ""
	m.result = ""
	return command
}

func (m *formModel) moveFocus(delta int) tea.Cmd {
	for index := range m.inputs {
		m.inputs[index].Blur()
	}
	m.notes.Blur()
	m.focus = (m.focus + delta + formControlCount) % formControlCount
	if m.focus < len(m.inputs) {
		return m.inputs[m.focus].Focus()
	}
	if m.focus == formNotes {
		return m.notes.Focus()
	}
	return nil
}

func (m *formModel) submit() {
	for index := range m.errors {
		m.errors[index] = ""
	}
	if strings.TrimSpace(m.inputs[formPurchaseOrder].Value()) == "" {
		m.errors[formPurchaseOrder] = "purchase order is required"
	}
	if strings.TrimSpace(m.inputs[formSKU].Value()) == "" {
		m.errors[formSKU] = "SKU is required"
	}
	quantity, err := strconv.Atoi(m.inputs[formQuantity].Value())
	if err != nil || quantity <= 0 {
		m.errors[formQuantity] = "quantity must be a positive number"
	}
	if strings.TrimSpace(m.inputs[formBin].Value()) == "" {
		m.errors[formBin] = "bin location is required"
	}

	firstInvalid := -1
	for index, validationError := range m.errors {
		if validationError != "" {
			firstInvalid = index
			break
		}
	}
	if firstInvalid >= 0 {
		m.result = "Correct the highlighted fields"
		m.focus = firstInvalid
		for index := range m.inputs {
			m.inputs[index].Blur()
		}
		m.notes.Blur()
		m.inputs[firstInvalid].Focus()
		return
	}

	m.submitted = true
	priority := "Normal"
	if m.priority == 1 {
		priority = "Urgent"
	}
	m.result = fmt.Sprintf(
		"Receipt submitted: %s / %s / quantity %d / %s / %s",
		m.inputs[formPurchaseOrder].Value(),
		m.inputs[formSKU].Value(),
		quantity,
		m.inputs[formBin].Value(),
		priority,
	)
}

func (m *formModel) resize(width, height int) {
	m.width = width
	m.height = height
	inputWidth := min(48, max(10, width-18))
	for index := range m.inputs {
		m.inputs[index].SetWidth(inputWidth)
	}
	m.notes.SetWidth(inputWidth)
}

func (m formModel) view() string {
	if m.submitted {
		return strings.Join([]string{
			headingStyle.Render("RECEIVING FORM"),
			"",
			successStyle.Render("RECEIPT ACCEPTED"),
			"",
			m.result,
			"",
			helpStyle.Render("Enter/Esc Return to menu    R Enter another receipt"),
		}, "\n")
	}

	lines := []string{headingStyle.Render("RECEIVING FORM"), ""}
	for index, input := range m.inputs {
		marker := "  "
		if m.focus == index {
			marker = "> "
		}
		line := fmt.Sprintf("%s%-15s %s", marker, formLabels[index]+":", input.View())
		lines = append(lines, line)
		if m.errors[index] != "" {
			lines = append(lines, "  "+errorStyle.Render(m.errors[index]))
		}
	}
	notesMarker := "  "
	if m.focus == formNotes {
		notesMarker = "> "
	}
	lines = append(lines, notesMarker+"Notes:", m.notes.View())

	priority := []string{"[x] Normal  [ ] Urgent", "[ ] Normal  [x] Urgent"}[m.priority]
	lines = append(lines, controlLine(m.focus == formPriority, "Priority:      "+priority))
	buttons := "[ Submit ]    [ Cancel ]"
	if m.focus == formSubmit {
		buttons = selectedStyle.Render("[ Submit ]") + "    [ Cancel ]"
	} else if m.focus == formCancel {
		buttons = "[ Submit ]    " + selectedStyle.Render("[ Cancel ]")
	}
	lines = append(lines, "", buttons)
	if m.result != "" {
		lines = append(lines, "", errorStyle.Render(m.result))
	}
	lines = append(lines, "", helpStyle.Render("Tab/Shift-Tab Move  Ctrl-N/P/Y SKU suggestions  Enter Select  Esc/F2 Menu"))
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}

func controlLine(focused bool, text string) string {
	if focused {
		return "> " + text
	}
	return "  " + text
}
