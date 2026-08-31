// Package reference implements the deterministic TUI used to exercise
// TUICast automation and terminal emulation.
package reference

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	loginUser     = "operator"
	loginPassword = "cast" + "ing"
)

type page uint8

const (
	pageLogin page = iota
	pageMenu
	pageForm
	pageTable
	pageScrolling
	pageColors
	pageCursor
	pageKeys
	pageResize
	pageUnicode
)

var menuItems = []string{
	"Login / Authentication",
	"Forms and Input Fields",
	"Tables",
	"Scrolling",
	"Colors and Attributes",
	"Cursor Movement",
	"Function Keys",
	"Partial Screen Updates",
	"Long Running Operation",
	"Terminal Resize",
	"Unicode",
	"ANSI / VT220 Tests",
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	headingStyle  = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	successStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
)

// Application is one isolated Bubble Tea reference application. Its pages,
// components, and deterministic scenario data are all compiled into the
// reference binary.
type Application struct {
	width       int
	height      int
	page        page
	loginFocus  int
	loginInputs [2]textinput.Model
	selected    int
	message     string
	form        formModel
	table       tableModel
	scrolling   scrollingModel
	colors      colorsModel
	cursor      cursorModel
	keys        keyModel
	resizeDemo  resizeModel
	unicode     unicodeModel
	vtTest      VTTestRunner
	runErr      error
}

// New creates a reference application with the requested terminal dimensions.
func New(width, height int) (*Application, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("creating reference TUI: dimensions must be positive")
	}

	username := textinput.New()
	username.Prompt = ""
	username.Placeholder = "operator"
	username.CharLimit = 24
	username.SetWidth(min(24, max(8, width-12)))
	password := textinput.New()
	password.Prompt = ""
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '*'
	password.CharLimit = 24
	password.SetWidth(username.Width())
	username.Focus()

	return &Application{
		width:       width,
		height:      height,
		loginInputs: [2]textinput.Model{username, password},
		form:        newFormModel(width, height),
		table:       newTableModel(width, height),
		scrolling:   newScrollingModel(width, height),
		colors:      newColorsModel(width, height),
		cursor:      newCursorModel(width, height),
		keys:        newKeyModel(width, height),
		resizeDemo:  newResizeModel(width, height),
		unicode:     newUnicodeModel(width, height),
		vtTest:      CommandVTTestRunner{},
	}, nil
}

// SetVTTestRunner replaces the external VTTEST launcher. Passing nil restores
// the default launcher.
func (a *Application) SetVTTestRunner(runner VTTestRunner) {
	if runner == nil {
		runner = CommandVTTestRunner{}
	}
	a.vtTest = runner
}

// Run serves one application session until the user exits or input ends.
func (a *Application) Run(input io.Reader, output io.Writer) error {
	if input == nil {
		return fmt.Errorf("running reference TUI: input is required")
	}
	if output == nil {
		return fmt.Errorf("running reference TUI: output is required")
	}

	program := tea.NewProgram(
		a,
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithWindowSize(a.width, a.height),
	)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("running reference TUI program: %w", err)
	}
	if a.runErr != nil {
		return fmt.Errorf("running reference TUI scenario: %w", a.runErr)
	}
	return nil
}

// Init implements tea.Model.
func (a *Application) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (a *Application) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		a.resize(message.Width, message.Height)
		return a, nil
	case vtTestFinishedMsg:
		return a, a.finishVTTest(message.err)
	case scrollTickMsg:
		if a.page == pageScrolling {
			_, command := a.scrolling.update(message)
			return a, command
		}
		return a, nil
	case tea.PasteMsg:
		return a, a.updatePaste(message)
	case tea.KeyPressMsg:
		if message.String() == "ctrl+c" && a.page != pageKeys {
			return a, tea.Quit
		}
		switch a.page {
		case pageLogin:
			return a, a.updateLogin(message)
		case pageMenu:
			return a, a.updateMenu(message)
		case pageForm:
			back, command := a.form.update(message)
			if back {
				a.page = pageMenu
				a.message = "Forms and Input Fields completed"
			}
			return a, command
		case pageTable:
			if a.table.update(message) {
				a.page = pageMenu
				a.message = "Tables completed"
			}
			return a, nil
		case pageScrolling:
			back, command := a.scrolling.update(message)
			if back {
				a.completeScenario("Scrolling")
			}
			return a, command
		case pageColors:
			if a.colors.update(message) {
				a.completeScenario("Colors and Attributes")
			}
			return a, nil
		case pageCursor:
			back, command := a.cursor.update(message)
			if back {
				a.completeScenario("Cursor Movement")
			}
			return a, command
		case pageKeys:
			if a.keys.update(message) {
				a.completeScenario("Function Keys")
			}
			return a, nil
		case pageResize:
			if a.resizeDemo.update(message) {
				a.completeScenario("Terminal Resize")
			}
			return a, nil
		case pageUnicode:
			if a.unicode.update(message) {
				a.completeScenario("Unicode")
			}
			return a, nil
		}
	}
	return a, nil
}

func (a *Application) updatePaste(message tea.PasteMsg) tea.Cmd {
	switch a.page {
	case pageLogin:
		var command tea.Cmd
		a.loginInputs[a.loginFocus], command = a.loginInputs[a.loginFocus].Update(message)
		a.message = ""
		return command
	case pageForm:
		return a.form.updatePaste(message)
	case pageTable:
		return a.table.updatePaste(message)
	case pageKeys:
		a.keys.updatePaste(message)
		return nil
	case pageResize:
		return a.resizeDemo.updatePaste(message)
	case pageUnicode:
		return a.unicode.updatePaste(message)
	default:
		return nil
	}
}

// View implements tea.Model.
func (a *Application) View() tea.View {
	var body string
	switch a.page {
	case pageLogin:
		body = a.viewLogin()
	case pageMenu:
		body = a.viewMenu()
	case pageForm:
		body = a.form.view()
	case pageTable:
		body = a.table.view()
	case pageScrolling:
		body = a.scrolling.view()
	case pageColors:
		body = a.colors.view()
	case pageCursor:
		body = a.cursor.view()
	case pageKeys:
		body = a.keys.view()
	case pageResize:
		body = a.resizeDemo.view()
	case pageUnicode:
		body = a.unicode.view()
	}
	view := tea.NewView(lipgloss.NewStyle().MaxWidth(a.width).Render(
		titleStyle.Render("TUICAST REFERENCE TERMINAL") + "\n\n" + body,
	))
	view.AltScreen = true
	if a.page == pageCursor && a.cursor.visible {
		view.Cursor = tea.NewCursor(a.cursor.column, a.cursor.row)
	}
	return view
}

func (a *Application) updateLogin(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "tab", "down":
		a.focusLogin((a.loginFocus + 1) % len(a.loginInputs))
		return textinput.Blink
	case "shift+tab", "up":
		a.focusLogin((a.loginFocus - 1 + len(a.loginInputs)) % len(a.loginInputs))
		return textinput.Blink
	case "enter":
		if a.loginFocus == 0 {
			a.focusLogin(1)
			return textinput.Blink
		}
		a.authenticate()
		return nil
	case "f1":
		a.authenticate()
		return nil
	case "f2":
		a.resetLogin("Login cleared")
		return nil
	}

	var command tea.Cmd
	a.loginInputs[a.loginFocus], command = a.loginInputs[a.loginFocus].Update(key)
	a.message = ""
	return command
}

func (a *Application) updateMenu(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "up":
		a.selected = (a.selected - 1 + len(menuItems)) % len(menuItems)
	case "down":
		a.selected = (a.selected + 1) % len(menuItems)
	case "f2":
		a.page = pageLogin
		a.resetLogin("Signed out")
	case "enter", "f1":
		switch a.selected {
		case 1:
			a.form = newFormModel(a.width, a.height)
			a.page = pageForm
		case 2:
			a.table = newTableModel(a.width, a.height)
			a.page = pageTable
		case 3:
			a.scrolling = newScrollingModel(a.width, a.height)
			a.page = pageScrolling
		case 4:
			a.colors = newColorsModel(a.width, a.height)
			a.page = pageColors
		case 5:
			a.cursor = newCursorModel(a.width, a.height)
			a.page = pageCursor
		case 6:
			a.keys = newKeyModel(a.width, a.height)
			a.page = pageKeys
		case 9:
			a.resizeDemo = newResizeModel(a.width, a.height)
			a.page = pageResize
		case 10:
			a.unicode = newUnicodeModel(a.width, a.height)
			a.page = pageUnicode
		case len(menuItems) - 1:
			a.message = "Launching VTTEST"
			return tea.Exec(&vtTestExecCommand{runner: a.vtTest}, func(err error) tea.Msg {
				return vtTestFinishedMsg{err: err}
			})
		default:
			a.message = menuItems[a.selected] + " scenario selected"
		}
	default:
		if characters := []rune(key.Text); len(characters) == 1 && characters[0] >= '1' && characters[0] <= '9' {
			a.selected = int(characters[0] - '1')
			a.message = menuItems[a.selected] + " scenario selected"
		}
	}
	return nil
}

func (a *Application) finishVTTest(err error) tea.Cmd {
	if err == nil {
		a.message = "VTTEST completed"
		return nil
	}
	var unavailable *vtTestUnavailableError
	if errors.As(err, &unavailable) {
		a.message = "VTTEST unavailable: " + unavailable.Unwrap().Error()
		return nil
	}
	a.runErr = fmt.Errorf("running VTTEST terminal lifecycle: %w", err)
	return tea.Quit
}

func (a *Application) completeScenario(name string) {
	a.page = pageMenu
	a.message = name + " completed"
}

func (a *Application) authenticate() {
	if a.loginInputs[0].Value() != loginUser || a.loginInputs[1].Value() != loginPassword {
		a.loginInputs[1].SetValue("")
		a.focusLogin(0)
		a.message = "Invalid user ID or password"
		return
	}
	a.loginInputs[0].Blur()
	a.loginInputs[1].Blur()
	a.page = pageMenu
	a.selected = 0
	a.message = "Authenticated as " + loginUser
}

func (a *Application) resetLogin(message string) {
	for index := range a.loginInputs {
		a.loginInputs[index].SetValue("")
	}
	a.focusLogin(0)
	a.message = message
}

func (a *Application) focusLogin(index int) {
	for inputIndex := range a.loginInputs {
		a.loginInputs[inputIndex].Blur()
	}
	a.loginFocus = index
	a.loginInputs[index].Focus()
}

func (a *Application) resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	a.width = width
	a.height = height
	inputWidth := min(24, max(8, width-12))
	for index := range a.loginInputs {
		a.loginInputs[index].SetWidth(inputWidth)
	}
	a.form.resize(width, height)
	a.table.resize(width, height)
	a.scrolling.resize(width, height)
	a.colors.resize(width, height)
	a.cursor.resize(width, height)
	a.keys.resize(width, height)
	a.resizeDemo.resize(width, height)
	a.unicode.resize(width, height)
}

func (a *Application) viewLogin() string {
	lines := []string{
		headingStyle.Render("LOGIN / AUTHENTICATION"),
		"",
		"User ID:  " + a.loginInputs[0].View(),
		"",
		"Password: " + a.loginInputs[1].View(),
		"",
		helpStyle.Render("F1 Login    F2 Clear    Ctrl-C Exit"),
		"",
		"Test credentials: operator / casting",
	}
	if a.message != "" {
		lines = append(lines, "", errorStyle.Render(a.message))
	}
	return strings.Join(lines, "\n")
}

func (a *Application) viewMenu() string {
	lines := []string{headingStyle.Render("TERMINAL TEST SYSTEM"), ""}
	for index, item := range menuItems {
		line := fmt.Sprintf("%2d. %s", index+1, item)
		if index == a.selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	if a.message != "" {
		lines = append(lines, "", successStyle.Render(a.message))
	}
	lines = append(lines, "", helpStyle.Render("Arrows Select  Enter/F1 Open  F2 Sign out  Ctrl-C Exit"))
	return strings.Join(lines, "\n")
}
