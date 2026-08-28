// Package reference implements the deterministic TUI used to exercise
// TUICast automation and terminal emulation.
package reference

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	loginUser     = "operator"
	loginPassword = "casting"
)

type page uint8

const (
	pageLogin page = iota
	pageMenu
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

// Application is one isolated reference-TUI session. It owns input decoding,
// screen state, and ANSI rendering.
type Application struct {
	width        int
	height       int
	page         page
	focus        int
	selected     int
	username     []rune
	password     []rune
	message      string
	decoder      inputDecoder
	opened       bool
	done         bool
	vtTest       VTTestRunner
	launchVTTest bool
}

// New creates a reference application with the requested terminal dimensions.
func New(width, height int) (*Application, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("creating reference TUI: dimensions must be positive")
	}
	return &Application{width: width, height: height, vtTest: CommandVTTestRunner{}}, nil
}

// SetVTTestRunner replaces the external VTTEST launcher. It is intended for
// embedding and deterministic tests; passing nil restores the default launcher.
func (a *Application) SetVTTestRunner(runner VTTestRunner) {
	if runner == nil {
		runner = CommandVTTestRunner{}
	}
	a.vtTest = runner
}

// Open enters the alternate screen and renders the initial login page.
func (a *Application) Open() []byte {
	if a.opened || a.done {
		return nil
	}
	a.opened = true
	return append([]byte("\x1b[?1049h"), a.render()...)
}

// Handle consumes an arbitrary fragment of terminal input and returns any
// resulting ANSI screen update. Escape and UTF-8 sequences may span calls.
func (a *Application) Handle(input []byte) []byte {
	if a.done {
		return nil
	}

	changed := false
	for _, event := range a.decoder.feed(input) {
		if a.apply(event) {
			changed = true
		}
		if a.done {
			return a.Close()
		}
	}
	if !changed {
		return nil
	}
	return a.render()
}

// Resize changes the dimensions used for subsequent renders and returns a
// complete render for the new size.
func (a *Application) Resize(width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("resizing reference TUI: dimensions must be positive")
	}
	a.width = width
	a.height = height
	if !a.opened || a.done {
		return nil, nil
	}
	return a.render(), nil
}

// Close restores the primary screen. It is idempotent.
func (a *Application) Close() []byte {
	if !a.opened {
		return nil
	}
	a.opened = false
	return []byte("\x1b[0m\x1b[?25h\x1b[?1049l")
}

// Done reports whether the user requested application termination.
func (a *Application) Done() bool {
	return a.done
}

// Run serves one application session until the user exits or input reaches
// end-of-file.
func (a *Application) Run(input io.Reader, output io.Writer) (runErr error) {
	if input == nil {
		return fmt.Errorf("running reference TUI: input is required")
	}
	if output == nil {
		return fmt.Errorf("running reference TUI: output is required")
	}
	if err := writeAll(output, a.Open()); err != nil {
		return fmt.Errorf("opening reference TUI: %w", err)
	}
	defer func() {
		if err := writeAll(output, a.Close()); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("closing reference TUI: %w", err))
		}
	}()

	buffer := make([]byte, 256)
	for !a.done {
		n, readErr := input.Read(buffer)
		if n > 0 {
			if err := writeAll(output, a.Handle(buffer[:n])); err != nil {
				return fmt.Errorf("rendering reference TUI: %w", err)
			}
			if a.launchVTTest {
				a.launchVTTest = false
				if err := writeAll(output, a.Close()); err != nil {
					return fmt.Errorf("suspending reference TUI for VTTEST: %w", err)
				}
				err := a.vtTest.Run(input, output)
				if err != nil {
					var fatal *FatalVTTestError
					if errors.As(err, &fatal) {
						return fmt.Errorf("running VTTEST terminal lifecycle: %w", fatal)
					}
					a.message = "VTTEST unavailable: " + err.Error()
				} else {
					a.message = "VTTEST completed"
				}
				if err := writeAll(output, a.Open()); err != nil {
					return fmt.Errorf("resuming reference TUI after VTTEST: %w", err)
				}
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return fmt.Errorf("reading reference TUI input: %w", readErr)
			}
			break
		}
	}
	return nil
}

func (a *Application) apply(event inputEvent) bool {
	if event.kind == inputExit {
		a.done = true
		return true
	}

	switch a.page {
	case pageLogin:
		return a.applyLogin(event)
	case pageMenu:
		return a.applyMenu(event)
	default:
		return false
	}
}

func (a *Application) applyLogin(event inputEvent) bool {
	switch event.kind {
	case inputText:
		field := a.activeField()
		if len(*field) >= 24 || !unicode.IsPrint(event.text) {
			return false
		}
		*field = append(*field, event.text)
		a.message = ""
		return true
	case inputBackspace:
		field := a.activeField()
		if len(*field) == 0 {
			return false
		}
		*field = (*field)[:len(*field)-1]
		a.message = ""
		return true
	case inputTab, inputUp, inputDown:
		a.focus = 1 - a.focus
		return true
	case inputEnter:
		if a.focus == 0 {
			a.focus = 1
			return true
		}
		return a.authenticate()
	case inputF1:
		return a.authenticate()
	case inputF2:
		a.username = nil
		a.password = nil
		a.focus = 0
		a.message = "Login cleared"
		return true
	default:
		return false
	}
}

func (a *Application) applyMenu(event inputEvent) bool {
	switch event.kind {
	case inputUp:
		if a.selected == 0 {
			a.selected = len(menuItems) - 1
		} else {
			a.selected--
		}
		return true
	case inputDown:
		a.selected = (a.selected + 1) % len(menuItems)
		return true
	case inputText:
		if event.text >= '1' && event.text <= '9' {
			index := int(event.text - '1')
			if index < len(menuItems) {
				a.selected = index
				a.message = menuItems[index] + " scenario selected"
				return true
			}
		}
		return false
	case inputEnter, inputF1:
		if a.selected == len(menuItems)-1 {
			a.message = "Launching VTTEST"
			a.launchVTTest = true
		} else {
			a.message = menuItems[a.selected] + " scenario selected"
		}
		return true
	case inputF2:
		a.page = pageLogin
		a.password = nil
		a.focus = 0
		a.message = "Signed out"
		return true
	default:
		return false
	}
}

func (a *Application) activeField() *[]rune {
	if a.focus == 0 {
		return &a.username
	}
	return &a.password
}

func (a *Application) authenticate() bool {
	if string(a.username) != loginUser || string(a.password) != loginPassword {
		a.password = nil
		a.focus = 0
		a.message = "Invalid user ID or password"
		return true
	}
	a.page = pageMenu
	a.selected = 0
	a.message = "Authenticated as " + loginUser
	return true
}

func (a *Application) render() []byte {
	var output strings.Builder
	output.WriteString("\x1b[?25l\x1b[2J\x1b[H\x1b[0m")
	a.writeLine(&output, 2, "\x1b[1;36mTUICAST REFERENCE TERMINAL\x1b[0m")

	switch a.page {
	case pageLogin:
		a.renderLogin(&output)
	case pageMenu:
		a.renderMenu(&output)
	}
	return []byte(output.String())
}

func (a *Application) renderLogin(output *strings.Builder) {
	a.writeLine(output, 4, "\x1b[1mLOGIN / AUTHENTICATION\x1b[0m")
	a.writeLine(output, 6, "User ID:  "+string(a.username))
	a.writeLine(output, 8, "Password: "+strings.Repeat("*", len(a.password)))
	a.writeLine(output, 11, "\x1b[33mF1\x1b[0m Login    \x1b[33mF2\x1b[0m Clear    \x1b[33mCtrl-C\x1b[0m Exit")
	a.writeLine(output, 13, "Test credentials: operator / casting")
	if a.message != "" {
		a.writeLine(output, 15, "\x1b[1;31m"+a.message+"\x1b[0m")
	}

	column := 11 + len(a.username)
	row := 6
	if a.focus == 1 {
		column = 11 + len(a.password)
		row = 8
	}
	a.writeCursor(output, column, row)
}

func (a *Application) renderMenu(output *strings.Builder) {
	a.writeLine(output, 4, "\x1b[1mTERMINAL TEST SYSTEM\x1b[0m")
	for index, item := range menuItems {
		line := fmt.Sprintf("%2d. %s", index+1, item)
		if index == a.selected {
			line = "\x1b[7m" + line + "\x1b[0m"
		}
		a.writeLine(output, 6+index, line)
	}
	if a.message != "" {
		a.writeLine(output, 19, "\x1b[1;32m"+a.message+"\x1b[0m")
	}
	a.writeLine(output, 21, "Arrow keys Select    Enter/F1 Open    F2 Sign out    Ctrl-C Exit")
	output.WriteString("\x1b[?25l")
}

func (a *Application) writeLine(output *strings.Builder, row int, text string) {
	if row < 1 || row > a.height {
		return
	}
	fmt.Fprintf(output, "\x1b[%d;1H%s", row, truncateANSI(text, a.width))
}

func (a *Application) writeCursor(output *strings.Builder, column, row int) {
	column = min(max(1, column), a.width)
	row = min(max(1, row), a.height)
	fmt.Fprintf(output, "\x1b[%d;%dH\x1b[?25h", row, column)
}

func truncateANSI(text string, width int) string {
	// Reference screens use ASCII content. Preserve escape sequences while
	// limiting printable bytes to the negotiated terminal width.
	printable := 0
	inEscape := false
	for index, character := range text {
		if character == '\x1b' {
			inEscape = true
		}
		if !inEscape {
			if printable == width {
				return text[:index] + "\x1b[0m"
			}
			printable++
		}
		if inEscape && character >= '@' && character <= '~' && character != '[' {
			inEscape = false
		}
	}
	return text
}

func writeAll(output io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := output.Write(data)
		data = data[written:]
		if err != nil {
			return fmt.Errorf("writing terminal output: %w", err)
		}
		if written == 0 {
			return fmt.Errorf("writing terminal output: %w", io.ErrShortWrite)
		}
	}
	return nil
}
