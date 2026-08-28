package xterm_test

import (
	"testing"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/xterm"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTerminal(t *testing.T) {
	Convey("An xterm terminal has the xterm-256color profile", t, func() {
		terminal, err := xterm.New(0, 24)
		So(err, ShouldNotBeNil)
		So(terminal, ShouldBeNil)

		terminal = newTerminal(8, 2)
		So(terminal.Profile(), ShouldEqual, tuicast.ProfileXTerm)
	})

	Convey("Private mode sequences may be split across writes", t, func() {
		terminal := newTerminal(8, 2)

		write(terminal, "\x1b[?25")
		So(terminal.Snapshot().Cursor.Visible, ShouldBeTrue)
		write(terminal, "l")
		So(terminal.Snapshot().Cursor.Visible, ShouldBeFalse)

		write(terminal, "\x1b[?25h")
		So(terminal.Snapshot().Cursor.Visible, ShouldBeTrue)
	})

	Convey("The 1049 alternate screen preserves the primary screen and cursor", t, func() {
		terminal := newTerminal(8, 2)
		write(terminal, "MAIN")

		write(terminal, "\x1b[?1049hALT\x1b[?25l")
		alternate := terminal.Snapshot()
		So(alternate.Line(0), ShouldEqual, "ALT     ")
		So(alternate.Cursor.Visible, ShouldBeFalse)

		write(terminal, "\x1b[?1049l")
		primary := terminal.Snapshot()
		So(primary.Line(0), ShouldEqual, "MAIN    ")
		So(primary.Cursor, ShouldResemble, tuicast.Cursor{Column: 4, Row: 0, Visible: false})
	})

	Convey("Resizing preserves both the primary and alternate screens", t, func() {
		terminal := newTerminal(4, 1)
		write(terminal, "AB\x1b[?47hXY")

		So(terminal.Resize(3, 2), ShouldBeNil)
		So(terminal.Snapshot().Text(), ShouldEqual, "XY \n   ")

		write(terminal, "\x1b[?47l")
		So(terminal.Snapshot().Text(), ShouldEqual, "AB \n   ")
	})

	Convey("The 256-color palette is represented by cell color indexes", t, func() {
		terminal := newTerminal(4, 1)

		write(terminal, "\x1b[38;5;196;48;5;22mX")
		cell, ok := terminal.Snapshot().CellAt(0, 0)

		So(ok, ShouldBeTrue)
		So(cell.Foreground, ShouldEqual, tuicast.Color(196))
		So(cell.Background, ShouldEqual, tuicast.Color(22))
	})

	Convey("Character insertion and deletion shift the current line", t, func() {
		terminal := newTerminal(8, 1)
		write(terminal, "abcdefgh\x1b[3G\x1b[2@")
		So(terminal.Snapshot().Line(0), ShouldEqual, "ab  cdef")

		terminal = newTerminal(8, 1)
		write(terminal, "abcdefgh\x1b[3G\x1b[2P")
		So(terminal.Snapshot().Line(0), ShouldEqual, "abefgh  ")
	})

	Convey("Line insertion and deletion operate inside the scrolling region", t, func() {
		terminal := newTerminal(4, 4)
		write(terminal, "1111\x1b[2;1H2222\x1b[3;1H3333\x1b[4;1H4444\x1b[2;1H\x1b[L")
		So(terminal.Snapshot().Text(), ShouldEqual, "1111\n    \n2222\n3333")

		terminal = newTerminal(4, 4)
		write(terminal, "1111\x1b[2;1H2222\x1b[3;1H3333\x1b[4;1H4444\x1b[2;1H\x1b[M")
		So(terminal.Snapshot().Text(), ShouldEqual, "1111\n3333\n4444\n    ")
	})

	Convey("Wide and combining characters retain cell geometry", t, func() {
		terminal := newTerminal(6, 1)
		write(terminal, "e\u0301界B")
		screen := terminal.Snapshot()

		first, _ := screen.CellAt(0, 0)
		wide, _ := screen.CellAt(1, 0)
		continuation, _ := screen.CellAt(2, 0)
		So(first.Text, ShouldEqual, "e\u0301")
		So(first.Width, ShouldEqual, 1)
		So(wide.Text, ShouldEqual, "界")
		So(wide.Width, ShouldEqual, 2)
		So(continuation.Text, ShouldEqual, "")
		So(continuation.Width, ShouldEqual, 0)
		So(screen.Line(0), ShouldEqual, "e\u0301界B  ")
	})

	Convey("Overwriting a wide-character continuation clears the full character", t, func() {
		terminal := newTerminal(4, 1)
		write(terminal, "界\x1b[2GX")

		So(terminal.Snapshot().Line(0), ShouldEqual, " X  ")
	})

	Convey("Snapshots remain detached from later writes", t, func() {
		terminal := newTerminal(4, 1)
		write(terminal, "A")
		before := terminal.Snapshot()
		write(terminal, "B")

		So(before.Line(0), ShouldEqual, "A   ")
		So(terminal.Snapshot().Line(0), ShouldEqual, "AB  ")
	})
}

func newTerminal(width, height int) *xterm.Terminal {
	terminal, err := xterm.New(width, height)
	So(err, ShouldBeNil)
	return terminal
}

func write(terminal *xterm.Terminal, transcript string) {
	n, err := terminal.Write([]byte(transcript))
	So(err, ShouldBeNil)
	So(n, ShouldEqual, len(transcript))
}
