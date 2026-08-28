package reference_test

import (
	"testing"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/reference"
	"github.com/castingcode/tuicast/xterm"
	. "github.com/smartystreets/goconvey/convey"
)

func TestReferenceTerminalCompatibility(t *testing.T) {
	Convey("The reference workflow renders through TUICast's xterm profile", t, func() {
		application, err := reference.New(80, 24)
		So(err, ShouldBeNil)
		terminal, err := xterm.New(80, 24)
		So(err, ShouldBeNil)

		writeTerminal(terminal, application.Open())
		So(terminal.Snapshot().Text(), ShouldContainSubstring, "LOGIN / AUTHENTICATION")

		writeTerminal(terminal, application.Handle([]byte("operator\tcasting\x1bOP")))
		screen := terminal.Snapshot()
		So(screen.Text(), ShouldContainSubstring, "TERMINAL TEST SYSTEM")
		So(screen.Text(), ShouldContainSubstring, "Authenticated as operator")
		So(screen.Cursor.Visible, ShouldBeFalse)

		writeTerminal(terminal, application.Handle([]byte("\x1bOQ")))
		screen = terminal.Snapshot()
		So(screen.Text(), ShouldContainSubstring, "LOGIN / AUTHENTICATION")
		So(screen.Cursor.Visible, ShouldBeTrue)
	})
}

func writeTerminal(terminal tuicast.Terminal, output []byte) {
	written, err := terminal.Write(output)
	So(err, ShouldBeNil)
	So(written, ShouldEqual, len(output))
}
