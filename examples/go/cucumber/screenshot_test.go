package main

import (
	"testing"

	tuicast "github.com/castingcode/tuicast/sdk/go"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTerminalCaptureHTML(t *testing.T) {
	Convey("A terminal capture preserves text, colors, attributes, and cursor position", t, func() {
		screen := tuicast.Screen{
			Width:  3,
			Height: 1,
			Cells: []tuicast.Cell{
				{Text: "<&", Width: 1, Foreground: tuicast.Red, Background: tuicast.DefaultColor, Attributes: tuicast.Bold | tuicast.Underline},
				{Text: "X", Width: 1, Foreground: tuicast.White, Background: tuicast.Color(196)},
				{Text: "Y", Width: 1, Foreground: tuicast.Green, Background: tuicast.Blue, Attributes: tuicast.Reverse},
			},
			Cursor:   tuicast.Cursor{Column: 2, Row: 0, Visible: true},
			Revision: 7,
		}

		capture := string(terminalCaptureHTML(screen))

		So(capture, ShouldContainSubstring, "Terminal 3x1 · revision 7")
		So(capture, ShouldContainSubstring, "&lt;&amp;")
		So(capture, ShouldContainSubstring, `font-weight="bold"`)
		So(capture, ShouldContainSubstring, `text-decoration="underline"`)
		So(capture, ShouldContainSubstring, `fill="#ff0000"`)
		So(capture, ShouldContainSubstring, `fill="#2472c8"`)
		So(capture, ShouldContainSubstring, `stroke="#ffffff"`)
	})
}
