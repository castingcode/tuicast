package reference

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/xterm"
	. "github.com/smartystreets/goconvey/convey"
)

func TestApplication(t *testing.T) {
	Convey("The Bubble Tea application starts on a deterministic login page", t, func() {
		application := newApplication()

		view := application.View().Content

		So(view, ShouldContainSubstring, "LOGIN / AUTHENTICATION")
		So(view, ShouldContainSubstring, "operator / casting")
		So(view, ShouldContainSubstring, "────────")
		So(application.loginInputs[0].Placeholder, ShouldBeEmpty)
		So(application.loginInputs[1].Placeholder, ShouldBeEmpty)
		So(application.loginInputs[0].Focused(), ShouldBeTrue)
	})

	Convey("Login supports focus navigation, validation, and function keys", t, func() {
		application := newApplication()
		application.Update(runes("wrong"))
		application.Update(key(tea.KeyTab))
		application.Update(runes("secret"))
		application.Update(key(tea.KeyF1))

		So(application.page, ShouldEqual, pageLogin)
		So(application.View().Content, ShouldContainSubstring, "Invalid user ID or password")
		So(application.loginInputs[1].Value(), ShouldBeEmpty)

		application.loginInputs[0].SetValue("")
		application.Update(runes("operator"))
		application.Update(key(tea.KeyTab))
		application.Update(runes("casting"))
		application.Update(key(tea.KeyF1))

		So(application.page, ShouldEqual, pageMenu)
		So(application.View().Content, ShouldContainSubstring, "TERMINAL TEST SYSTEM")
	})

	Convey("Bracketed paste reaches active input components and the key inspector", t, func() {
		application := newApplication()
		application.Update(tea.PasteMsg{Content: "operator"})
		application.Update(key(tea.KeyTab))
		application.Update(tea.PasteMsg{Content: "casting"})
		application.Update(key(tea.KeyF1))
		So(application.page, ShouldEqual, pageMenu)

		application.selected = 1
		application.Update(key(tea.KeyEnter))
		application.Update(tea.PasteMsg{Content: "PO-PASTED"})
		So(application.form.inputs[formPurchaseOrder].Value(), ShouldEqual, "PO-PASTED")

		application.page = pageKeys
		application.Update(tea.PasteMsg{Content: "bulk input"})
		So(application.keys.view(), ShouldContainSubstring, `paste "bulk input"`)
	})

	Convey("The menu opens forms and tables as pages of the same application", t, func() {
		application := authenticatedApplication()
		application.selected = 1

		application.Update(key(tea.KeyEnter))
		So(application.page, ShouldEqual, pageForm)
		So(application.View().Content, ShouldContainSubstring, "RECEIVING FORM")

		application.Update(key(tea.KeyEsc))
		application.selected = 2
		application.Update(key(tea.KeyEnter))
		So(application.page, ShouldEqual, pageTable)
		So(application.View().Content, ShouldContainSubstring, "WAREHOUSE ORDERS")
	})

	Convey("The menu opens every implemented terminal laboratory", t, func() {
		cases := []struct {
			selection int
			page      page
			text      string
		}{
			{3, pageScrolling, "DETERMINISTIC EVENT LOG"},
			{4, pageColors, "ANSI COLORS AND ATTRIBUTES"},
			{5, pageCursor, "CURSOR MOVEMENT"},
			{6, pageKeys, "FUNCTION KEYS"},
			{7, pagePartial, "PARTIAL SCREEN UPDATES"},
			{8, pageLongRunning, "LONG-RUNNING OPERATION"},
			{9, pageResize, "TERMINAL RESIZE"},
			{10, pageUnicode, "UNICODE ALIGNMENT"},
			{11, pageBellENQ, "BELL / ENQ ANSWERBACK"},
		}
		for _, testCase := range cases {
			application := authenticatedApplication()
			application.selected = testCase.selection

			application.Update(key(tea.KeyEnter))

			So(application.page, ShouldEqual, testCase.page)
			So(application.View().Content, ShouldContainSubstring, testCase.text)
		}
	})

	Convey("BELL and ENQ emit once and capture answerback in event order", t, func() {
		application := authenticatedApplication()
		application.selected = 11
		application.Update(key(tea.KeyEnter))

		_, bell := application.Update(runes("b"))
		_, duplicateBell := application.Update(runes("b"))
		_, enq := application.Update(runes("e"))
		repeatedENQKey := runes("e")
		repeatedENQKey.IsRepeat = true
		_, duplicateENQ := application.Update(repeatedENQKey)
		application.Update(runes("TUICAST-ANSWER"))

		So(bell, ShouldNotBeNil)
		So(bell().(tea.RawMsg).Msg, ShouldEqual, bellByte)
		So(enq, ShouldNotBeNil)
		So(enq().(tea.RawMsg).Msg, ShouldEqual, enqByte)
		So(duplicateBell, ShouldBeNil)
		So(duplicateENQ, ShouldBeNil)
		So(application.bellENQ.bellCount, ShouldEqual, 1)
		So(application.bellENQ.enqCount, ShouldEqual, 1)
		So(application.bellENQ.answerback, ShouldEqual, "TUICAST-ANSWER")
		So(application.View().Content, ShouldContainSubstring, "BEL emitted: 1 / 1")
		So(application.View().Content, ShouldContainSubstring, `Answerback: "TUICAST-ANSWER"`)
		So(application.bellENQ.events, ShouldResemble, []string{
			"1. BEL emitted",
			"2. ENQ emitted",
			`3. Answerback received: "TUICAST-ANSWER"`,
		})
	})

	Convey("The form validates required values and accepts a complete receipt", t, func() {
		application := authenticatedApplication()
		application.selected = 1
		application.Update(key(tea.KeyEnter))
		application.form.focus = formSubmit

		application.Update(key(tea.KeyEnter))

		So(application.form.focus, ShouldEqual, formPurchaseOrder)
		So(application.View().Content, ShouldContainSubstring, "purchase order is required")
		So(application.View().Content, ShouldContainSubstring, "quantity must be a positive number")

		application.form.inputs[formPurchaseOrder].SetValue("PO-10002341")
		application.form.inputs[formSKU].SetValue("WIDGET-42")
		application.form.inputs[formQuantity].SetValue("25")
		application.form.inputs[formBin].SetValue("A-01-02")
		application.form.notes.SetValue("dock 3\ninspect packaging")
		application.form.priority = 1
		application.form.focus = formSubmit
		application.Update(key(tea.KeyEnter))

		So(application.form.submitted, ShouldBeTrue)
		So(application.View().Content, ShouldContainSubstring, "RECEIPT ACCEPTED")
		So(application.View().Content, ShouldContainSubstring, "PO-10002341 / WIDGET-42 / quantity 25 / A-01-02 / Urgent")
	})

	Convey("The form supports reverse focus traversal and preserves values on resize", t, func() {
		form := newFormModel(80, 24)
		form.inputs[0].SetValue("PO-10002341")

		_, _ = form.update(modifiedKey(tea.KeyTab, tea.ModShift))
		So(form.focus, ShouldEqual, formCancel)

		form.resize(45, 18)
		So(form.inputs[0].Value(), ShouldEqual, "PO-10002341")
		So(form.inputs[0].Width(), ShouldEqual, 27)

		form.focus = formNotes
		form.notes.Focus()
		_, _ = form.update(runes("first line"))
		_, _ = form.update(key(tea.KeyEnter))
		_, _ = form.update(runes("second line"))
		So(form.notes.Value(), ShouldEqual, "first line\nsecond line")

		form.inputs[formPurchaseOrder].Blur()
		form.focus = formSKU
		form.inputs[formSKU].Focus()
		_, _ = form.update(runes("WID"))
		_, _ = form.update(modifiedKey('y', tea.ModCtrl))
		So(form.inputs[formSKU].Value(), ShouldEqual, "WIDGET-42")
	})

	Convey("The table navigates, filters, sorts, shows details, and adapts columns", t, func() {
		orders := newTableModel(80, 24)
		So(orders.table.Rows(), ShouldHaveLength, 30)

		orders.update(key(tea.KeyDown))
		So(orders.table.Cursor(), ShouldEqual, 1)
		orders.update(key(tea.KeyPgDown))
		So(orders.table.Cursor(), ShouldBeGreaterThan, 1)
		orders.update(key(tea.KeyHome))
		So(orders.table.Cursor(), ShouldEqual, 0)
		orders.update(key(tea.KeyDown))
		orders.update(key(tea.KeyEnter))
		So(orders.view(), ShouldContainSubstring, "ORDER DETAILS")
		So(orders.view(), ShouldContainSubstring, "ORD-10002342")
		orders.update(key(tea.KeyEsc))

		orders.update(runes("/"))
		orders.update(runes("SHIPPED"))
		orders.update(key(tea.KeyEnter))
		So(orders.table.Rows(), ShouldHaveLength, 7)
		for _, row := range orders.table.Rows() {
			So(row[2], ShouldEqual, "SHIPPED")
		}

		orders.update(runes("/"))
		orders.update(key(tea.KeyEsc))
		orders.update(runes("s"))
		So(orders.sortField, ShouldEqual, 1)
		So(orders.table.Rows()[0][2], ShouldEqual, "ALLOCATED")

		orders.resize(48, 16)
		So(orders.columns, ShouldEqual, 3)
		So(orders.table.Rows()[0], ShouldHaveLength, 3)
	})

	Convey("Scrolling supports navigation, follow mode, and deterministic delayed streaming", t, func() {
		log := newScrollingModel(80, 24)
		So(log.events, ShouldHaveLength, 100)

		_, _ = log.update(key(tea.KeyEnd))
		So(log.cursor, ShouldEqual, 99)
		_, _ = log.update(runes("f"))
		_, _ = log.update(runes("a"))
		So(log.cursor, ShouldEqual, 100)

		_, command := log.update(runes("s"))
		So(command, ShouldNotBeNil)
		generation := log.generation
		for range 5 {
			_, command = log.update(scrollTickMsg{generation: generation})
		}
		So(command, ShouldBeNil)
		So(log.events, ShouldHaveLength, 106)
		So(log.status, ShouldEqual, "STREAM COMPLETE (5 EVENTS)")
		So(log.cursor, ShouldEqual, 105)
	})

	Convey("Colors expose labeled indexed palettes and attributes without true color", t, func() {
		view := newColorsModel(132, 24).view()

		So(view, ShouldContainSubstring, "ANSI 8 COLORS (background)")
		So(view, ShouldContainSubstring, "\x1b[48;5;196m")
		So(view, ShouldContainSubstring, "BOLD")
		So(view, ShouldContainSubstring, "CONCEAL")
		So(view, ShouldNotContainSubstring, "38;2;")

		terminal, err := xterm.New(132, 24)
		So(err, ShouldBeNil)
		_, err = terminal.Write([]byte(view))
		So(err, ShouldBeNil)
		found := false
		for row := 0; row < 24; row++ {
			column := strings.Index(terminal.Snapshot().Line(row), "196")
			if column < 0 {
				continue
			}
			cell, ok := terminal.Snapshot().CellAt(column, row)
			So(ok, ShouldBeTrue)
			So(cell.Foreground, ShouldEqual, tuicast.Color(15))
			So(cell.Background, ShouldEqual, tuicast.Color(196))
			found = true
			break
		}
		So(found, ShouldBeTrue)
	})

	Convey("Cursor movement emits queryable position and visibility state", t, func() {
		cursor := newCursorModel(80, 24)
		So(cursor.column, ShouldEqual, 39)
		So(cursor.row, ShouldEqual, 11)

		_, _ = cursor.update(key(tea.KeyRight))
		_, _ = cursor.update(key(tea.KeyF1))
		_, _ = cursor.update(key(tea.KeyDown))
		_, _ = cursor.update(key(tea.KeyF2))
		So(cursor.column, ShouldEqual, 40)
		So(cursor.row, ShouldEqual, 11)
		application := newApplication()
		application.page = pageCursor
		application.cursor = cursor
		So(application.View().Cursor.X, ShouldEqual, 40)
		So(application.View().Cursor.Y, ShouldEqual, 11)

		_, _ = cursor.update(key(tea.KeyF3))
		application.cursor = cursor
		So(application.View().Cursor, ShouldBeNil)
	})

	Convey("The key inspector reports function-key and modifier conventions", t, func() {
		inspector := newKeyModel(80, 24)
		inspector.update(key(tea.KeyF1))
		inspector.update(key(tea.KeyF13))
		inspector.update(modifiedKey(tea.KeyUp, tea.ModAlt))
		inspector.update(modifiedKey('a', tea.ModCtrl))
		inspector.updatePaste(tea.PasteMsg{Content: "résumé"})

		So(inspector.count, ShouldEqual, 5)
		So(inspector.history[1], ShouldContainSubstring, "shift+f1 convention")
		So(inspector.history[2], ShouldContainSubstring, "alt+up")
		So(inspector.history[3], ShouldContainSubstring, "ctrl+a")
		So(inspector.view(), ShouldContainSubstring, `paste "résumé"`)
	})

	Convey("Partial updates expose premature readiness and gate input until completion", t, func() {
		partial := newPartialModel(80, 24)
		_, command := partial.update(runes("1"))
		So(command, ShouldNotBeNil)

		_, _ = partial.update(partialTickMsg{generation: partial.generation})
		So(partial.view(), ShouldContainSubstring, "READY")
		So(partial.view(), ShouldNotContainSubstring, "SCREEN COMPLETE")
		_, _ = partial.update(runes("x"))
		So(partial.ignored, ShouldEqual, 1)

		for partial.running {
			_, _ = partial.update(partialTickMsg{generation: partial.generation})
		}
		So(partial.view(), ShouldContainSubstring, "SCREEN COMPLETE / INPUT ENABLED")
		_, _ = partial.update(runes("A"))
		So(partial.accepted, ShouldEqual, "A")
		So(partial.view(), ShouldContainSubstring, "Ignored early key events: 1")
	})

	Convey("Partial updates exercise independent regions and in-place progress", t, func() {
		partial := newPartialModel(80, 24)
		_, _ = partial.update(runes("2"))
		for partial.running {
			_, _ = partial.update(partialTickMsg{generation: partial.generation})
		}
		So(partial.view(), ShouldContainSubstring, "WAVE-17 ACTIVE")
		So(partial.view(), ShouldContainSubstring, "ORD-10002342 / SHIPPED")
		So(partial.view(), ShouldContainSubstring, "REGIONS COMPLETE")

		_, _ = partial.update(runes("3"))
		for partial.running {
			_, _ = partial.update(partialTickMsg{generation: partial.generation})
		}
		So(partial.view(), ShouldContainSubstring, "[##########] 100%")
		So(partial.view(), ShouldContainSubstring, "Status: COMPLETE")
	})

	Convey("The finite operation completes and preserves deterministic totals", t, func() {
		operation := newLongRunningModel(80, 24)
		_, command := operation.update(runes("1"))
		So(command, ShouldNotBeNil)
		for operation.running {
			_, _ = operation.update(longRunningTickMsg{generation: operation.generation})
		}

		So(operation.heartbeats, ShouldEqual, 40)
		So(operation.processed, ShouldEqual, 120)
		So(operation.succeeded, ShouldEqual, 116)
		So(operation.failed, ShouldEqual, 4)
		So(operation.remaining, ShouldEqual, 0)
		So(operation.status, ShouldEqual, "OPERATION COMPLETE")
		So(operation.events, ShouldHaveLength, 8)
	})

	Convey("The continuous operation pauses, resumes, bounds history, and stops", t, func() {
		operation := newLongRunningModel(80, 24)
		_, _ = operation.update(runes("2"))
		staleGeneration := operation.generation
		_, _ = operation.update(runes("p"))
		_, _ = operation.update(longRunningTickMsg{generation: staleGeneration})
		So(operation.processed, ShouldEqual, 0)
		So(operation.status, ShouldEqual, "OPERATION PAUSED")

		_, command := operation.update(runes("P"))
		So(command, ShouldNotBeNil)
		for range 40 {
			_, _ = operation.update(longRunningTickMsg{generation: operation.generation})
		}
		So(operation.events, ShouldHaveLength, 8)
		operation.resize(132, 24)
		So(operation.processed, ShouldEqual, 40)
		_, _ = operation.update(runes("s"))
		So(operation.status, ShouldEqual, "SESSION STOPPED")
		So(operation.running, ShouldBeFalse)
	})

	Convey("The failure operation stops at its documented error", t, func() {
		operation := newLongRunningModel(80, 24)
		_, _ = operation.update(runes("3"))
		for operation.running {
			_, _ = operation.update(longRunningTickMsg{generation: operation.generation})
		}

		So(operation.processed, ShouldEqual, 60)
		So(operation.succeeded, ShouldEqual, 59)
		So(operation.failed, ShouldEqual, 1)
		So(operation.remaining, ShouldEqual, 40)
		So(operation.status, ShouldEqual, "OPERATION FAILED / E-WAVE-060")
	})

	Convey("Resize toggles between 80x24 and 132x24 while retaining state", t, func() {
		resize := newResizeModel(80, 24)
		resize.input.SetValue("keep me")
		resize.update(key(tea.KeyDown))

		resize.update(runes("t"))
		So(resize.targetWidth, ShouldEqual, 132)
		So(resize.view(), ShouldContainSubstring, "WAITING")
		resize.resize(132, 24)
		So(resize.view(), ShouldContainSubstring, "MATCH")
		So(resize.input.Value(), ShouldEqual, "keep me")
		So(resizeOrders[resize.selected], ShouldEqual, "ORD-10002342")

		resize.update(runes("T"))
		resize.resize(80, 24)
		So(resize.view(), ShouldContainSubstring, "Target dimensions: 80x24")
		So(resize.view(), ShouldContainSubstring, "MATCH")
	})

	Convey("Unicode samples and editable graphemes survive resizing", t, func() {
		unicode := newUnicodeModel(80, 24)
		unicode.update(runes("e\u0301漢🙂"))
		So(unicode.view(), ShouldContainSubstring, "Expected width")
		unicode.resize(45, 18)

		So(unicode.input.Value(), ShouldEqual, "e\u0301漢🙂")
		So(unicode.view(), ShouldContainSubstring, "Combining accent")
		So(unicode.view(), ShouldContainSubstring, "👩‍💻")
	})

	Convey("VTTEST is launched through Bubble Tea's terminal handoff", t, func() {
		application := authenticatedApplication()
		runner := &recordingVTTestRunner{}
		application.SetVTTestRunner(runner)
		application.selected = len(menuItems) - 1

		_, command := application.Update(key(tea.KeyEnter))

		So(command, ShouldNotBeNil)
		So(runner.calls, ShouldEqual, 0)

		var output bytes.Buffer
		executable := &vtTestExecCommand{runner: runner}
		executable.SetStdin(strings.NewReader(""))
		executable.SetStdout(&output)
		So(executable.Run(), ShouldBeNil)
		So(output.String(), ShouldEqual, "[VTTEST]")
	})

	Convey("VTTEST availability and fatal lifecycle errors remain distinct", t, func() {
		application := newApplication()

		command := &vtTestExecCommand{runner: &recordingVTTestRunner{err: fmt.Errorf("not installed")}}
		command.SetStdin(strings.NewReader(""))
		command.SetStdout(io.Discard)
		err := command.Run()
		application.finishVTTest(err)
		So(application.message, ShouldContainSubstring, "VTTEST unavailable: not installed")
		So(application.runErr, ShouldBeNil)

		fatal := &FatalVTTestError{Err: fmt.Errorf("terminal restore failed")}
		application.finishVTTest(fatal)
		So(application.runErr, ShouldNotBeNil)
	})

	Convey("Run delegates terminal lifecycle to Bubble Tea", t, func() {
		application := newApplication()
		var output bytes.Buffer

		err := application.Run(&pacedReader{fragments: [][]byte{[]byte("\x03")}}, &output)

		So(err, ShouldBeNil)
		So(output.String(), ShouldContainSubstring, "\x1b[?1049h")
		So(output.String(), ShouldContainSubstring, "LOGIN / AUTHENTICATION")
		So(output.String(), ShouldContainSubstring, "\x1b[?1049l")
	})
}

func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func runes(value string) tea.KeyPressMsg {
	code := tea.KeyExtended
	if characters := []rune(value); len(characters) == 1 {
		code = characters[0]
	}
	return tea.KeyPressMsg{Code: code, Text: value}
}

func modifiedKey(code rune, modifier tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: modifier}
}

type recordingVTTestRunner struct {
	calls int
	err   error
}

type pacedReader struct {
	fragments [][]byte
}

func (r *pacedReader) Read(data []byte) (int, error) {
	time.Sleep(200 * time.Millisecond)
	if len(r.fragments) == 0 {
		return 0, io.EOF
	}
	fragment := r.fragments[0]
	r.fragments = r.fragments[1:]
	return copy(data, fragment), nil
}

func (r *recordingVTTestRunner) Run(_ io.Reader, output io.Writer) error {
	r.calls++
	if _, err := output.Write([]byte("[VTTEST]")); err != nil {
		return fmt.Errorf("writing recorded VTTEST output: %w", err)
	}
	return r.err
}

func newApplication() *Application {
	application, err := New(80, 24)
	So(err, ShouldBeNil)
	return application
}

func authenticatedApplication() *Application {
	application := newApplication()
	application.loginInputs[0].SetValue(loginUser)
	application.loginInputs[1].SetValue(loginPassword)
	application.authenticate()
	So(application.page, ShouldEqual, pageMenu)
	return application
}
