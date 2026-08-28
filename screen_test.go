package tuicast_test

import (
	"testing"

	"github.com/castingcode/tuicast"
	. "github.com/smartystreets/goconvey/convey"
)

func TestScreen(t *testing.T) {
	Convey("A screen exposes exact row-major contents", t, func() {
		screen := tuicast.Screen{
			Width:  3,
			Height: 2,
			Cells: []tuicast.Cell{
				{Text: "A", Width: 1},
				{Text: "界", Width: 2},
				{Width: 0},
				{Text: "x", Width: 1},
				{Text: " ", Width: 1},
				{Text: " ", Width: 1},
			},
		}

		So(screen.Line(0), ShouldEqual, "A界")
		So(screen.Line(1), ShouldEqual, "x  ")
		So(screen.Text(), ShouldEqual, "A界\nx  ")

		cell, ok := screen.CellAt(1, 0)
		So(ok, ShouldBeTrue)
		So(cell.Text, ShouldEqual, "界")

		_, ok = screen.CellAt(3, 0)
		So(ok, ShouldBeFalse)
	})

	Convey("Screen matchers compose text, row, and cursor expectations", t, func() {
		screen := tuicast.Screen{
			Width:  5,
			Height: 1,
			Cells: []tuicast.Cell{
				{Text: "R", Width: 1},
				{Text: "E", Width: 1},
				{Text: "A", Width: 1},
				{Text: "D", Width: 1},
				{Text: "Y", Width: 1},
			},
			Cursor: tuicast.Cursor{Column: 4, Row: 0},
		}

		ready := tuicast.ScreenLineEquals(0, "READY")
		cursor := tuicast.CursorAt(4, 0)
		So(tuicast.AllOf(ready, cursor).Match(screen), ShouldBeTrue)
		So(tuicast.AnyOf(tuicast.ScreenContains("missing"), cursor).Match(screen), ShouldBeTrue)
		So(tuicast.Not(tuicast.CursorAt(0, 0)).Match(screen), ShouldBeTrue)
	})
}
