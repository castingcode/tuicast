// Package ansi provides shared ANSI/DEC terminal emulation for concrete
// terminal profiles.
package ansi

import (
	"fmt"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/parsing"
	xansi "github.com/charmbracelet/x/ansi"
)

// Profile selects a concrete set of terminal behavior.
type Profile uint8

const (
	VT220 Profile = iota
	XTerm
)

var _ tuicast.KeyEncoder = (*Terminal)(nil)
var _ tuicast.ModifiedKeyEncoder = (*Terminal)(nil)
var _ tuicast.TerminalEventDrainer = (*Terminal)(nil)
var _ tuicast.AnswerbackTerminal = (*Terminal)(nil)

type screenBuffer struct {
	cells       []tuicast.Cell
	cursor      tuicast.Cursor
	savedCursor tuicast.Cursor
}

// Terminal is the shared ANSI/DEC emulation engine.
type Terminal struct {
	mu sync.RWMutex

	terminalType      tuicast.TerminalProfile
	profileName       string
	xterm             bool
	width             int
	height            int
	main              screenBuffer
	alternate         screenBuffer
	active            *screenBuffer
	style             tuicast.Cell
	revision          uint64
	wrapPending       bool
	autoWrap          bool
	applicationCursor bool
	insertMode        bool
	scrollTop         int
	scrollBottom      int
	charsets          [2]bool
	charset           int
	parser            *parsing.Parser
	answerback        string
	events            []tuicast.TerminalEvent
}

// New creates a terminal engine for profile.
func New(width, height int, profile Profile) (*Terminal, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("creating terminal: dimensions must be positive")
	}

	terminalType, profileName, xterm, err := profileSettings(profile)
	if err != nil {
		return nil, err
	}
	terminal := &Terminal{
		terminalType: terminalType,
		profileName:  profileName,
		xterm:        xterm,
		width:        width,
		height:       height,
		style:        defaultCell(),
		autoWrap:     true,
		scrollBottom: height - 1,
	}
	terminal.main = screenBuffer{
		cells:  terminal.blankCells(width * height),
		cursor: tuicast.Cursor{Visible: true},
	}
	terminal.active = &terminal.main
	terminal.parser = parsing.New(parsing.Handler{
		Print:   terminal.print,
		Execute: terminal.execute,
		CSI:     terminal.handleCSI,
		Escape:  terminal.handleEscape,
	})
	return terminal, nil
}

func profileSettings(profile Profile) (tuicast.TerminalProfile, string, bool, error) {
	switch profile {
	case VT220:
		return tuicast.ProfileVT220, "VT220", false, nil
	case XTerm:
		return tuicast.ProfileXTerm, "xterm", true, nil
	default:
		return "", "", false, fmt.Errorf("creating terminal: unknown profile %d", profile)
	}
}

// Profile returns the terminal behavior being emulated.
func (t *Terminal) Profile() tuicast.TerminalProfile {
	return t.terminalType
}

// SetAnswerback configures the printable response to an ENQ control. DEC
// terminals limit the answerback message to 20 bytes.
func (t *Terminal) SetAnswerback(answerback string) error {
	if len(answerback) > 20 {
		return fmt.Errorf("configuring %s answerback: message exceeds 20 bytes", t.profileName)
	}
	for _, character := range []byte(answerback) {
		if character < 0x20 || character > 0x7e {
			return fmt.Errorf("configuring %s answerback: message must contain printable ASCII", t.profileName)
		}
	}
	t.mu.Lock()
	t.answerback = answerback
	t.mu.Unlock()
	return nil
}

// DrainEvents returns and clears terminal control events produced by Write.
func (t *Terminal) DrainEvents() []tuicast.TerminalEvent {
	t.mu.Lock()
	defer t.mu.Unlock()
	events := append([]tuicast.TerminalEvent(nil), t.events...)
	t.events = nil
	return events
}

// EncodeKey returns the input sequence for a named key.
func (t *Terminal) EncodeKey(key tuicast.Key) ([]byte, error) {
	return t.EncodeKeyPress(tuicast.KeyPress{Key: key})
}

// EncodeKeyPress returns the input sequence for a key and its modifiers.
func (t *Terminal) EncodeKeyPress(press tuicast.KeyPress) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if press.Modifiers&^(tuicast.ModifierShift|tuicast.ModifierAlt|tuicast.ModifierControl) != 0 {
		return nil, fmt.Errorf("encoding %s key: unsupported modifiers %d", t.profileName, press.Modifiers)
	}
	if utf8.RuneCountInString(string(press.Key)) == 1 {
		return t.encodePrintableKey(press)
	}
	if press.Modifiers == 0 && t.applicationCursor {
		applicationSequences := map[tuicast.Key]string{
			tuicast.KeyUp:    "\x1bOA",
			tuicast.KeyDown:  "\x1bOB",
			tuicast.KeyRight: "\x1bOC",
			tuicast.KeyLeft:  "\x1bOD",
		}
		if sequence, ok := applicationSequences[press.Key]; ok {
			return []byte(sequence), nil
		}
	}
	if press.Modifiers != 0 {
		if sequence, ok := modifiedNamedKey(press); ok {
			return []byte(sequence), nil
		}
		return nil, fmt.Errorf("encoding %s key: modifiers %d are unsupported for key %q", t.profileName, press.Modifiers, press.Key)
	}

	sequences := map[tuicast.Key]string{
		tuicast.KeyEnter:     "\r",
		tuicast.KeyTab:       "\t",
		tuicast.KeyBackspace: "\x7f",
		tuicast.KeyEscape:    "\x1b",
		tuicast.KeyUp:        "\x1b[A",
		tuicast.KeyDown:      "\x1b[B",
		tuicast.KeyRight:     "\x1b[C",
		tuicast.KeyLeft:      "\x1b[D",
		tuicast.KeyHome:      "\x1b[H",
		tuicast.KeyEnd:       "\x1b[F",
		tuicast.KeyInsert:    "\x1b[2~",
		tuicast.KeyDelete:    "\x1b[3~",
		tuicast.KeyPageUp:    "\x1b[5~",
		tuicast.KeyPageDown:  "\x1b[6~",
		tuicast.KeyF1:        "\x1bOP",
		tuicast.KeyF2:        "\x1bOQ",
		tuicast.KeyF3:        "\x1bOR",
		tuicast.KeyF4:        "\x1bOS",
		tuicast.KeyF5:        "\x1b[15~",
		tuicast.KeyF6:        "\x1b[17~",
		tuicast.KeyF7:        "\x1b[18~",
		tuicast.KeyF8:        "\x1b[19~",
		tuicast.KeyF9:        "\x1b[20~",
		tuicast.KeyF10:       "\x1b[21~",
		tuicast.KeyF11:       "\x1b[23~",
		tuicast.KeyF12:       "\x1b[24~",
	}
	sequence, ok := sequences[press.Key]
	if !ok {
		return nil, fmt.Errorf("encoding %s key: unsupported key %q", t.profileName, press.Key)
	}
	return []byte(sequence), nil
}

func (t *Terminal) encodePrintableKey(press tuicast.KeyPress) ([]byte, error) {
	key, _ := utf8.DecodeRuneInString(string(press.Key))
	if press.Modifiers&tuicast.ModifierShift != 0 {
		key = unicode.ToUpper(key)
	}
	encoded := string(key)
	if press.Modifiers&tuicast.ModifierControl != 0 {
		control, ok := controlCharacter(key)
		if !ok {
			return nil, fmt.Errorf("encoding %s key: Control is unsupported for %q", t.profileName, press.Key)
		}
		encoded = string([]byte{control})
	}
	if press.Modifiers&tuicast.ModifierAlt != 0 {
		encoded = "\x1b" + encoded
	}
	return []byte(encoded), nil
}

func controlCharacter(key rune) (byte, bool) {
	if key >= 'a' && key <= 'z' {
		key = unicode.ToUpper(key)
	}
	switch {
	case key == ' ' || key == '@':
		return 0x00, true
	case key >= 'A' && key <= 'Z':
		return byte(key - 'A' + 1), true
	case key >= '[' && key <= '_':
		return byte(key - '[' + 0x1b), true
	case key == '?':
		return 0x7f, true
	default:
		return 0, false
	}
}

func modifiedNamedKey(press tuicast.KeyPress) (string, bool) {
	modifier := 1 + int(press.Modifiers)
	finals := map[tuicast.Key]string{
		tuicast.KeyUp:    "A",
		tuicast.KeyDown:  "B",
		tuicast.KeyRight: "C",
		tuicast.KeyLeft:  "D",
		tuicast.KeyHome:  "H",
		tuicast.KeyEnd:   "F",
		tuicast.KeyF1:    "P",
		tuicast.KeyF2:    "Q",
		tuicast.KeyF3:    "R",
		tuicast.KeyF4:    "S",
	}
	if final, ok := finals[press.Key]; ok {
		return fmt.Sprintf("\x1b[1;%d%s", modifier, final), true
	}
	tildeCodes := map[tuicast.Key]string{
		tuicast.KeyInsert:   "2",
		tuicast.KeyDelete:   "3",
		tuicast.KeyPageUp:   "5",
		tuicast.KeyPageDown: "6",
		tuicast.KeyF5:       "15",
		tuicast.KeyF6:       "17",
		tuicast.KeyF7:       "18",
		tuicast.KeyF8:       "19",
		tuicast.KeyF9:       "20",
		tuicast.KeyF10:      "21",
		tuicast.KeyF11:      "23",
		tuicast.KeyF12:      "24",
	}
	if code, ok := tildeCodes[press.Key]; ok {
		return fmt.Sprintf("\x1b[%s;%d~", code, modifier), true
	}
	if press.Key == tuicast.KeyTab && press.Modifiers == tuicast.ModifierShift {
		return "\x1b[Z", true
	}
	if press.Key == tuicast.KeyEnter || press.Key == tuicast.KeyBackspace || press.Key == tuicast.KeyEscape {
		base := map[tuicast.Key]string{
			tuicast.KeyEnter:     "\r",
			tuicast.KeyBackspace: "\x7f",
			tuicast.KeyEscape:    "\x1b",
		}[press.Key]
		if press.Modifiers&tuicast.ModifierAlt != 0 {
			base = "\x1b" + base
		}
		if press.Modifiers&^(tuicast.ModifierAlt|tuicast.ModifierShift) == 0 {
			return base, true
		}
	}
	return "", false
}

// Write applies host output to the terminal.
func (t *Terminal) Write(data []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	before := t.revision
	n, err := t.parser.Write(data)
	if err != nil {
		return n, fmt.Errorf("parsing %s output: %w", t.profileName, err)
	}
	if t.revision != before {
		t.revision = before + 1
	}
	return n, nil
}

// Snapshot returns state detached from subsequent terminal updates.
func (t *Terminal) Snapshot() tuicast.Screen {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cells := make([]tuicast.Cell, len(t.active.cells))
	copy(cells, t.active.cells)
	return tuicast.Screen{
		Width:    t.width,
		Height:   t.height,
		Cells:    cells,
		Cursor:   t.active.cursor,
		Revision: t.revision,
	}
}

// Resize changes the terminal dimensions while preserving each buffer's
// overlapping upper-left region.
func (t *Terminal) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("resizing %s terminal: dimensions must be positive", t.profileName)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if width == t.width && height == t.height {
		return nil
	}

	t.resizeBuffer(&t.main, width, height)
	if t.alternate.cells != nil {
		t.resizeBuffer(&t.alternate, width, height)
	}
	t.width = width
	t.height = height
	t.scrollTop = 0
	t.scrollBottom = height - 1
	t.wrapPending = false
	t.revision++
	return nil
}

func (t *Terminal) resizeBuffer(buffer *screenBuffer, width, height int) {
	cells := t.blankCells(width * height)
	copyWidth := min(width, t.width)
	copyHeight := min(height, t.height)
	for row := 0; row < copyHeight; row++ {
		copy(cells[row*width:row*width+copyWidth], buffer.cells[row*t.width:row*t.width+copyWidth])
		t.normalizeCells(cells[row*width : (row+1)*width])
	}
	buffer.cells = cells
	buffer.cursor.Column = min(buffer.cursor.Column, width-1)
	buffer.cursor.Row = min(buffer.cursor.Row, height-1)
	buffer.savedCursor.Column = min(buffer.savedCursor.Column, width-1)
	buffer.savedCursor.Row = min(buffer.savedCursor.Row, height-1)
}

func defaultCell() tuicast.Cell {
	return tuicast.Cell{
		Text:       " ",
		Width:      1,
		Foreground: tuicast.DefaultColor,
		Background: tuicast.DefaultColor,
	}
}

func (t *Terminal) blankCells(count int) []tuicast.Cell {
	cells := make([]tuicast.Cell, count)
	for i := range cells {
		cells[i] = t.blankCell()
	}
	return cells
}

func (t *Terminal) blankCell() tuicast.Cell {
	cell := t.style
	cell.Text = " "
	cell.Width = 1
	return cell
}

func (t *Terminal) changed() {
	t.revision++
}

func (t *Terminal) print(r rune) {
	if t.charsets[t.charset] {
		r = decSpecialGraphics(r)
	}

	width := 1
	if t.xterm {
		width = xansi.StringWidth(string(r))
		if width < 0 || width > 2 {
			width = 1
		}
	}
	if width == 0 {
		t.appendCombining(r)
		return
	}
	if width > t.width {
		width = 1
	}
	if t.wrapPending || (width == 2 && t.active.cursor.Column == t.width-1 && t.autoWrap) {
		if t.autoWrap {
			t.active.cursor.Column = 0
			t.lineFeed()
		}
		t.wrapPending = false
	}
	if width == 2 && t.active.cursor.Column == t.width-1 {
		width = 1
	}
	if t.insertMode {
		t.insertCharacters(width)
	}

	column := t.active.cursor.Column
	row := t.active.cursor.Row
	t.clearWideCharacterAt(row, column)
	if width == 2 {
		t.clearWideCharacterAt(row, column+1)
	}
	cell := t.style
	cell.Text = string(r)
	cell.Width = width
	t.active.cells[row*t.width+column] = cell
	if width == 2 {
		continuation := t.style
		continuation.Text = ""
		continuation.Width = 0
		t.active.cells[row*t.width+column+1] = continuation
	}

	end := column + width
	if end >= t.width {
		t.active.cursor.Column = t.width - 1
		t.wrapPending = t.autoWrap
	} else {
		t.active.cursor.Column = end
	}
	t.changed()
}

func (t *Terminal) appendCombining(r rune) {
	column := t.active.cursor.Column - 1
	if t.wrapPending {
		column = t.active.cursor.Column
	}
	if column < 0 {
		return
	}
	index := t.active.cursor.Row*t.width + column
	for index >= 0 && t.active.cells[index].Width == 0 {
		index--
	}
	if index >= 0 {
		t.active.cells[index].Text += string(r)
		t.changed()
	}
}

func (t *Terminal) clearWideCharacterAt(row, column int) {
	if column < 0 || column >= t.width {
		return
	}
	line := t.row(row)
	switch line[column].Width {
	case 0:
		if column > 0 && line[column-1].Width == 2 {
			line[column-1] = t.blankCell()
		}
		line[column] = t.blankCell()
	case 2:
		line[column] = t.blankCell()
		if column+1 < t.width && line[column+1].Width == 0 {
			line[column+1] = t.blankCell()
		}
	}
}

func (t *Terminal) normalizeCells(cells []tuicast.Cell) {
	for column := range cells {
		switch cells[column].Width {
		case 0:
			if column == 0 || cells[column-1].Width != 2 {
				cells[column] = t.blankCell()
			}
		case 2:
			if column+1 >= len(cells) || cells[column+1].Width != 0 {
				cells[column] = t.blankCell()
			}
		}
	}
}

func (t *Terminal) execute(control byte) {
	switch control {
	case 0x05:
		t.events = append(t.events, tuicast.TerminalEvent{Type: tuicast.EventEnquiry, Data: t.answerback})
	case 0x07:
		t.events = append(t.events, tuicast.TerminalEvent{Type: tuicast.EventBell})
	case '\b':
		t.active.cursor.Column = max(0, t.active.cursor.Column-1)
		t.wrapPending = false
		t.changed()
	case '\t':
		t.active.cursor.Column = min(((t.active.cursor.Column/8)+1)*8, t.width-1)
		t.wrapPending = false
		t.changed()
	case '\n', '\v', '\f':
		t.lineFeed()
		t.wrapPending = false
	case '\r':
		t.active.cursor.Column = 0
		t.wrapPending = false
		t.changed()
	case 0x0e:
		t.charset = 1
	case 0x0f:
		t.charset = 0
	}
}

func (t *Terminal) lineFeed() {
	if t.active.cursor.Row == t.scrollBottom {
		t.scrollUp()
	} else if t.active.cursor.Row < t.height-1 {
		t.active.cursor.Row++
	}
	t.changed()
}

func (t *Terminal) scrollUp() {
	for row := t.scrollTop; row < t.scrollBottom; row++ {
		copy(t.row(row), t.row(row+1))
	}
	t.clearRow(t.scrollBottom)
}

func (t *Terminal) reverseIndex() {
	if t.active.cursor.Row == t.scrollTop {
		for row := t.scrollBottom; row > t.scrollTop; row-- {
			copy(t.row(row), t.row(row-1))
		}
		t.clearRow(t.scrollTop)
	} else if t.active.cursor.Row > 0 {
		t.active.cursor.Row--
	}
	t.changed()
}

func (t *Terminal) row(row int) []tuicast.Cell {
	start := row * t.width
	return t.active.cells[start : start+t.width]
}

func (t *Terminal) clearRow(row int) {
	copy(t.row(row), t.blankCells(t.width))
}

func (t *Terminal) handleEscape(command parsing.Command) {
	switch {
	case command.Intermediate == '(' && (command.Final == '0' || command.Final == 'B'):
		t.charsets[0] = command.Final == '0'
	case command.Intermediate == ')' && (command.Final == '0' || command.Final == 'B'):
		t.charsets[1] = command.Final == '0'
	case command.Final == '7':
		t.active.savedCursor = t.active.cursor
	case command.Final == '8':
		t.active.cursor = t.active.savedCursor
		t.wrapPending = false
		t.changed()
	case command.Final == 'D':
		t.lineFeed()
	case command.Final == 'E':
		t.active.cursor.Column = 0
		t.lineFeed()
	case command.Final == 'M':
		t.reverseIndex()
	case command.Final == 'c':
		t.reset()
	}
}

func (t *Terminal) handleCSI(command parsing.Command, parameters []parsing.Parameter) {
	if command.Prefix == '?' && (command.Final == 'h' || command.Final == 'l') {
		enabled := command.Final == 'h'
		t.setCursorKeyMode(parameters, enabled)
		if t.xterm {
			t.setPrivateModes(parameters, enabled)
		}
		return
	}
	if command.Prefix != 0 {
		return
	}

	switch command.Final {
	case '@':
		if t.xterm {
			t.insertCharacters(param(parameters, 0, 1))
		}
	case 'A':
		t.active.cursor.Row = max(0, t.active.cursor.Row-param(parameters, 0, 1))
		t.cursorChanged()
	case 'B':
		t.active.cursor.Row = min(t.height-1, t.active.cursor.Row+param(parameters, 0, 1))
		t.cursorChanged()
	case 'C':
		t.active.cursor.Column = min(t.width-1, t.active.cursor.Column+param(parameters, 0, 1))
		t.cursorChanged()
	case 'D':
		t.active.cursor.Column = max(0, t.active.cursor.Column-param(parameters, 0, 1))
		t.cursorChanged()
	case 'G':
		t.active.cursor.Column = min(t.width-1, max(0, param(parameters, 0, 1)-1))
		t.cursorChanged()
	case 'H', 'f':
		t.active.cursor.Row = min(t.height-1, max(0, param(parameters, 0, 1)-1))
		t.active.cursor.Column = min(t.width-1, max(0, param(parameters, 1, 1)-1))
		t.cursorChanged()
	case 'J':
		t.eraseDisplay(param(parameters, 0, 0))
	case 'K':
		t.eraseLine(param(parameters, 0, 0))
	case 'L':
		if t.xterm {
			t.insertLines(param(parameters, 0, 1))
		}
	case 'M':
		if t.xterm {
			t.deleteLines(param(parameters, 0, 1))
		}
	case 'P':
		if t.xterm {
			t.deleteCharacters(param(parameters, 0, 1))
		}
	case 'X':
		if t.xterm {
			t.eraseCharacters(param(parameters, 0, 1))
		}
	case 'h', 'l':
		if t.xterm {
			t.setModes(parameters, command.Final == 'h')
		}
	case 'm':
		t.setRendition(parameters)
	case 'r':
		t.setScrollRegion(parameters)
	}
}

func (t *Terminal) cursorChanged() {
	t.wrapPending = false
	t.changed()
}

func param(parameters []parsing.Parameter, index, fallback int) int {
	if index >= len(parameters) || !parameters[index].Present || parameters[index].Value == 0 {
		return fallback
	}
	return parameters[index].Value
}

func (t *Terminal) eraseDisplay(mode int) {
	cursor := t.active.cursor.Row*t.width + t.active.cursor.Column
	switch mode {
	case 0:
		for i := cursor; i < len(t.active.cells); i++ {
			t.active.cells[i] = t.blankCell()
		}
	case 1:
		for i := 0; i <= cursor; i++ {
			t.active.cells[i] = t.blankCell()
		}
	case 2:
		for i := range t.active.cells {
			t.active.cells[i] = t.blankCell()
		}
	case 3:
		if !t.xterm {
			return
		}
		for i := range t.active.cells {
			t.active.cells[i] = t.blankCell()
		}
	default:
		return
	}
	t.changed()
}

func (t *Terminal) eraseLine(mode int) {
	start := t.active.cursor.Row * t.width
	end := start + t.width
	switch mode {
	case 0:
		start += t.active.cursor.Column
	case 1:
		end = start + t.active.cursor.Column + 1
	case 2:
	default:
		return
	}
	for i := start; i < end; i++ {
		t.active.cells[i] = t.blankCell()
	}
	t.changed()
}

func (t *Terminal) eraseCharacters(count int) {
	count = min(max(1, count), t.width-t.active.cursor.Column)
	start := t.active.cursor.Row*t.width + t.active.cursor.Column
	for i := start; i < start+count; i++ {
		t.active.cells[i] = t.blankCell()
	}
	t.changed()
}

func (t *Terminal) insertCharacters(count int) {
	count = min(max(1, count), t.width-t.active.cursor.Column)
	line := t.row(t.active.cursor.Row)
	column := t.active.cursor.Column
	copy(line[column+count:], line[column:t.width-count])
	copy(line[column:column+count], t.blankCells(count))
	t.normalizeCells(line)
	t.changed()
}

func (t *Terminal) deleteCharacters(count int) {
	count = min(max(1, count), t.width-t.active.cursor.Column)
	line := t.row(t.active.cursor.Row)
	column := t.active.cursor.Column
	copy(line[column:], line[column+count:])
	copy(line[t.width-count:], t.blankCells(count))
	t.normalizeCells(line)
	t.changed()
}

func (t *Terminal) insertLines(count int) {
	row := t.active.cursor.Row
	if row < t.scrollTop || row > t.scrollBottom {
		return
	}
	count = min(max(1, count), t.scrollBottom-row+1)
	for destination := t.scrollBottom; destination >= row+count; destination-- {
		copy(t.row(destination), t.row(destination-count))
	}
	for blank := row; blank < row+count; blank++ {
		t.clearRow(blank)
	}
	t.changed()
}

func (t *Terminal) deleteLines(count int) {
	row := t.active.cursor.Row
	if row < t.scrollTop || row > t.scrollBottom {
		return
	}
	count = min(max(1, count), t.scrollBottom-row+1)
	for destination := row; destination <= t.scrollBottom-count; destination++ {
		copy(t.row(destination), t.row(destination+count))
	}
	for blank := t.scrollBottom - count + 1; blank <= t.scrollBottom; blank++ {
		t.clearRow(blank)
	}
	t.changed()
}

func (t *Terminal) setModes(parameters []parsing.Parameter, enabled bool) {
	for _, parameter := range parameters {
		if parameter.Present && parameter.Value == 4 {
			t.insertMode = enabled
		}
	}
}

func (t *Terminal) setCursorKeyMode(parameters []parsing.Parameter, enabled bool) {
	for _, parameter := range parameters {
		if parameter.Present && parameter.Value == 1 {
			t.applicationCursor = enabled
		}
	}
}

func (t *Terminal) setPrivateModes(parameters []parsing.Parameter, enabled bool) {
	for _, parameter := range parameters {
		if !parameter.Present {
			continue
		}
		switch parameter.Value {
		case 7:
			t.autoWrap = enabled
			t.wrapPending = false
		case 25:
			t.main.cursor.Visible = enabled
			if t.alternate.cells != nil {
				t.alternate.cursor.Visible = enabled
			}
			t.changed()
		case 47, 1047:
			if enabled {
				t.enterAlternate(parameter.Value == 1047)
			} else {
				t.exitAlternate()
			}
		case 1049:
			if enabled {
				t.main.savedCursor = t.main.cursor
				t.enterAlternate(true)
			} else {
				t.exitAlternate()
				visible := t.main.cursor.Visible
				t.main.cursor = t.main.savedCursor
				t.main.cursor.Visible = visible
			}
		}
	}
}

func (t *Terminal) enterAlternate(clear bool) {
	if t.active == &t.alternate {
		if clear {
			t.alternate.cells = t.blankCells(t.width * t.height)
			t.alternate.cursor = tuicast.Cursor{Visible: true}
		}
		return
	}
	if t.alternate.cells == nil || clear {
		t.alternate = screenBuffer{
			cells:  t.blankCells(t.width * t.height),
			cursor: tuicast.Cursor{Visible: t.main.cursor.Visible},
		}
	}
	t.active = &t.alternate
	t.wrapPending = false
	t.changed()
}

func (t *Terminal) exitAlternate() {
	if t.active != &t.alternate {
		return
	}
	t.active = &t.main
	t.wrapPending = false
	t.changed()
}

func (t *Terminal) setRendition(parameters []parsing.Parameter) {
	if len(parameters) == 0 {
		parameters = []parsing.Parameter{{Value: 0, Present: true}}
	}

	for i := 0; i < len(parameters); i++ {
		parameter := parameters[i]
		value := parameter.Value
		if !parameter.Present {
			value = 0
		}
		switch {
		case value == 0:
			t.style = defaultCell()
		case value == 1:
			t.style.Attributes |= tuicast.Bold
		case value == 4:
			t.style.Attributes |= tuicast.Underline
		case value == 5:
			t.style.Attributes |= tuicast.Blink
		case value == 7:
			t.style.Attributes |= tuicast.Reverse
		case value == 8:
			t.style.Attributes |= tuicast.Conceal
		case value == 22:
			t.style.Attributes &^= tuicast.Bold
		case value == 24:
			t.style.Attributes &^= tuicast.Underline
		case value == 25:
			t.style.Attributes &^= tuicast.Blink
		case value == 27:
			t.style.Attributes &^= tuicast.Reverse
		case value == 28:
			t.style.Attributes &^= tuicast.Conceal
		case value >= 30 && value <= 37:
			t.style.Foreground = tuicast.Color(value - 30)
		case value == 38:
			i = t.setExtendedColor(parameters, i, true)
		case value == 39:
			t.style.Foreground = tuicast.DefaultColor
		case value >= 40 && value <= 47:
			t.style.Background = tuicast.Color(value - 40)
		case value == 48:
			i = t.setExtendedColor(parameters, i, false)
		case value == 49:
			t.style.Background = tuicast.DefaultColor
		case t.xterm && value >= 90 && value <= 97:
			t.style.Foreground = tuicast.Color(value - 90 + 8)
		case t.xterm && value >= 100 && value <= 107:
			t.style.Background = tuicast.Color(value - 100 + 8)
		}
	}
}

func (t *Terminal) setExtendedColor(parameters []parsing.Parameter, index int, foreground bool) int {
	if !t.xterm || index+2 >= len(parameters) || parameters[index+1].Value != 5 || !parameters[index+2].Present {
		return index
	}
	color := parameters[index+2].Value
	if color < 0 || color > 255 {
		return index + 2
	}
	if foreground {
		t.style.Foreground = tuicast.Color(color)
	} else {
		t.style.Background = tuicast.Color(color)
	}
	return index + 2
}

func (t *Terminal) setScrollRegion(parameters []parsing.Parameter) {
	top := param(parameters, 0, 1) - 1
	bottom := param(parameters, 1, t.height) - 1
	if top < 0 || bottom >= t.height || top >= bottom {
		return
	}
	t.scrollTop = top
	t.scrollBottom = bottom
	t.active.cursor.Column = 0
	t.active.cursor.Row = 0
	t.wrapPending = false
	t.changed()
}

func (t *Terminal) reset() {
	t.style = defaultCell()
	t.main = screenBuffer{
		cells:  t.blankCells(t.width * t.height),
		cursor: tuicast.Cursor{Visible: true},
	}
	t.alternate = screenBuffer{}
	t.active = &t.main
	t.scrollTop = 0
	t.scrollBottom = t.height - 1
	t.wrapPending = false
	t.autoWrap = true
	t.applicationCursor = false
	t.insertMode = false
	t.charsets = [2]bool{}
	t.charset = 0
	t.changed()
}

func decSpecialGraphics(r rune) rune {
	characters := map[rune]rune{
		'j': '┘', 'k': '┐', 'l': '┌', 'm': '└', 'n': '┼',
		'q': '─', 't': '├', 'u': '┤', 'v': '┴', 'w': '┬', 'x': '│',
	}
	if mapped, ok := characters[r]; ok {
		return mapped
	}
	return r
}
