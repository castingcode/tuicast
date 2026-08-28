package vt220_test

import (
	"testing"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTerminal(t *testing.T) {
	Convey("A VT220 terminal requires positive dimensions", t, func() {
		terminal, err := vt220.New(0, 24)
		So(err, ShouldNotBeNil)
		So(terminal, ShouldBeNil)
	})

	Convey("A representative application transcript updates screen state", t, func() {
		terminal := newTerminal(20, 4)

		write(terminal, "\x1b[2J\x1b[2;3HOrder \x1b[1;31mREADY\x1b[0m")
		screen := terminal.Snapshot()

		So(screen.Line(1), ShouldEqual, "  Order READY       ")
		So(screen.Cursor, ShouldResemble, tuicast.Cursor{Column: 13, Row: 1, Visible: true})
		cell, ok := screen.CellAt(8, 1)
		So(ok, ShouldBeTrue)
		So(cell.Text, ShouldEqual, "R")
		So(cell.Foreground, ShouldEqual, tuicast.Red)
		So(cell.Attributes.Has(tuicast.Bold), ShouldBeTrue)
	})

	Convey("Escape sequences may be split across network reads", t, func() {
		terminal := newTerminal(8, 2)

		write(terminal, "\x1b[2;")
		So(terminal.Snapshot().Revision, ShouldEqual, uint64(0))
		write(terminal, "4HOK")

		So(terminal.Snapshot().Line(1), ShouldEqual, "   OK   ")
	})

	Convey("Arrow keys follow DEC application cursor mode", t, func() {
		terminal := newTerminal(8, 2)

		key, err := terminal.EncodeKey(tuicast.KeyUp)
		So(err, ShouldBeNil)
		So(string(key), ShouldEqual, "\x1b[A")

		write(terminal, "\x1b[?1h")
		key, err = terminal.EncodeKey(tuicast.KeyUp)
		So(err, ShouldBeNil)
		So(string(key), ShouldEqual, "\x1bOA")

		write(terminal, "\x1b[?1l")
		key, err = terminal.EncodeKey(tuicast.KeyUp)
		So(err, ShouldBeNil)
		So(string(key), ShouldEqual, "\x1b[A")
	})

	Convey("Printable and named keys encode combined modifiers", t, func() {
		terminal := newTerminal(8, 2)

		key, err := terminal.EncodeKeyPress(tuicast.KeyPress{
			Key:       tuicast.Key("c"),
			Modifiers: tuicast.ModifierControl,
		})
		So(err, ShouldBeNil)
		So(key, ShouldResemble, []byte{0x03})

		key, err = terminal.EncodeKeyPress(tuicast.KeyPress{
			Key:       tuicast.Key("a"),
			Modifiers: tuicast.ModifierShift | tuicast.ModifierMeta,
		})
		So(err, ShouldBeNil)
		So(string(key), ShouldEqual, "\x1bA")

		key, err = terminal.EncodeKeyPress(tuicast.KeyPress{
			Key:       tuicast.KeyUp,
			Modifiers: tuicast.ModifierShift | tuicast.ModifierControl,
		})
		So(err, ShouldBeNil)
		So(string(key), ShouldEqual, "\x1b[1;6A")

		key, err = terminal.EncodeKeyPress(tuicast.KeyPress{
			Key:       tuicast.KeyTab,
			Modifiers: tuicast.ModifierShift,
		})
		So(err, ShouldBeNil)
		So(string(key), ShouldEqual, "\x1b[Z")
	})

	Convey("BELL and ENQ emit events without changing the screen", t, func() {
		terminal := newTerminal(8, 2)
		So(terminal.SetAnswerback("TUICAST"), ShouldBeNil)
		before := terminal.Snapshot()

		write(terminal, "\a\x05")
		events := terminal.DrainEvents()

		So(events, ShouldResemble, []tuicast.TerminalEvent{
			{Type: tuicast.EventBell},
			{Type: tuicast.EventEnquiry, Data: "TUICAST"},
		})
		So(terminal.Snapshot(), ShouldResemble, before)
		So(terminal.SetAnswerback("not\nprintable"), ShouldNotBeNil)
	})

	Convey("Text wraps and the bottom margin scrolls", t, func() {
		terminal := newTerminal(4, 2)

		write(terminal, "abcde")
		So(terminal.Snapshot().Text(), ShouldEqual, "abcd\ne   ")

		write(terminal, "\r\nnext\r\n")
		So(terminal.Snapshot().Text(), ShouldEqual, "next\n    ")
	})

	Convey("Erasure respects the cursor and requested direction", t, func() {
		terminal := newTerminal(6, 2)

		write(terminal, "abcdef\x1b[2;1Huvwxyz\x1b[1;3H\x1b[K\x1b[2;3H\x1b[1K")

		So(terminal.Snapshot().Text(), ShouldEqual, "ab    \n   xyz")
	})

	Convey("Scrolling can be restricted to top and bottom margins", t, func() {
		terminal := newTerminal(3, 4)

		write(terminal, "111\x1b[2;1H222\x1b[3;1H333\x1b[4;1H444")
		write(terminal, "\x1b[2;3r\x1b[3;1H\n")

		So(terminal.Snapshot().Text(), ShouldEqual, "111\n333\n   \n444")
	})

	Convey("DEC special graphics render line-drawing characters", t, func() {
		terminal := newTerminal(6, 1)

		write(terminal, "\x1b(0lqk\x1b(B")

		So(terminal.Snapshot().Line(0), ShouldEqual, "┌─┐   ")
	})

	Convey("Snapshots remain detached from later writes", t, func() {
		terminal := newTerminal(4, 1)
		write(terminal, "A")
		before := terminal.Snapshot()

		write(terminal, "B")
		after := terminal.Snapshot()

		So(before.Line(0), ShouldEqual, "A   ")
		So(after.Line(0), ShouldEqual, "AB  ")
		So(before.Revision, ShouldEqual, uint64(1))
		So(after.Revision, ShouldEqual, uint64(2))
	})

	Convey("Resizing preserves the overlapping screen region", t, func() {
		terminal := newTerminal(4, 2)
		write(terminal, "AB\r\nCD")

		err := terminal.Resize(3, 3)
		So(err, ShouldBeNil)
		screen := terminal.Snapshot()

		So(screen.Width, ShouldEqual, 3)
		So(screen.Height, ShouldEqual, 3)
		So(screen.Text(), ShouldEqual, "AB \nCD \n   ")
	})
}

func newTerminal(width, height int) *vt220.Terminal {
	terminal, err := vt220.New(width, height)
	So(err, ShouldBeNil)
	return terminal
}

func write(terminal *vt220.Terminal, transcript string) {
	n, err := terminal.Write([]byte(transcript))
	So(err, ShouldBeNil)
	So(n, ShouldEqual, len(transcript))
}
