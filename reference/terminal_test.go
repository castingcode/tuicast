package reference_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/castingcode/tuicast/reference"
	"github.com/castingcode/tuicast/xterm"
	. "github.com/smartystreets/goconvey/convey"
)

func TestReferenceTerminalCompatibility(t *testing.T) {
	Convey("Bubble Tea renders the reference workflow through TUICast's xterm profile", t, func() {
		application, err := reference.New(80, 24)
		So(err, ShouldBeNil)
		var transcript bytes.Buffer

		err = application.Run(&pacedReader{fragments: [][]byte{
			[]byte("operator\tcasting\x1bOP"),
			[]byte("\x03"),
		}}, &transcript)
		So(err, ShouldBeNil)

		terminal, err := xterm.New(80, 24)
		So(err, ShouldBeNil)
		written, err := terminal.Write(transcript.Bytes())
		So(err, ShouldBeNil)
		So(written, ShouldEqual, transcript.Len())
		So(transcript.String(), ShouldContainSubstring, "TERMINAL TEST SYSTEM")
		So(terminal.Snapshot().Cursor.Visible, ShouldBeTrue)
	})

	Convey("The form workflow is reachable through terminal input", t, func() {
		application, err := reference.New(80, 24)
		So(err, ShouldBeNil)
		var transcript bytes.Buffer

		err = application.Run(&pacedReader{fragments: [][]byte{
			[]byte("operator\tcasting\x1bOP"),
			[]byte("\x1b[B\r"),
			[]byte("\x03"),
		}}, &transcript)

		So(err, ShouldBeNil)
		So(transcript.String(), ShouldContainSubstring, "RECEIVING FORM")
	})

	Convey("The table and order details are reachable through terminal input", t, func() {
		application, err := reference.New(80, 24)
		So(err, ShouldBeNil)
		var transcript bytes.Buffer

		err = application.Run(&pacedReader{fragments: [][]byte{
			[]byte("operator\tcasting\x1bOP"),
			[]byte("\x1b[B\x1b[B\r"),
			[]byte("\x1b[B\r"),
			[]byte("\x03"),
		}}, &transcript)

		So(err, ShouldBeNil)
		So(transcript.String(), ShouldContainSubstring, "ORDER DETAILS")
		So(transcript.String(), ShouldContainSubstring, "ORD-10002342")
	})

	Convey("The cursor laboratory positions the terminal's real cursor", t, func() {
		application, err := reference.New(80, 24)
		So(err, ShouldBeNil)
		var transcript bytes.Buffer

		err = application.Run(&pacedReader{fragments: [][]byte{
			[]byte("operator\tcasting\x1bOP"),
			[]byte("\x1b[B\x1b[B\x1b[B\x1b[B\x1b[B\r"),
			[]byte("\x03"),
		}}, &transcript)
		So(err, ShouldBeNil)
		So(transcript.String(), ShouldContainSubstring, "CURSOR MOVEMENT")
		beforeExit := strings.LastIndex(transcript.String(), "\x1b[>4m")
		So(beforeExit, ShouldBeGreaterThan, 0)

		terminal, err := xterm.New(80, 24)
		So(err, ShouldBeNil)
		_, err = terminal.Write(transcript.Bytes()[:beforeExit])
		So(err, ShouldBeNil)
		snapshot := terminal.Snapshot()
		So(snapshot.Cursor.Column, ShouldEqual, 39)
		So(snapshot.Cursor.Row, ShouldEqual, 11)
		So(snapshot.Cursor.Visible, ShouldBeTrue)
	})
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
