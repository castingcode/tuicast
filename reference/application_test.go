package reference

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestApplication(t *testing.T) {
	Convey("The application opens on a deterministic login screen", t, func() {
		application := newApplication()

		output := string(application.Open())

		So(output, ShouldContainSubstring, "\x1b[?1049h")
		So(output, ShouldContainSubstring, "LOGIN / AUTHENTICATION")
		So(output, ShouldContainSubstring, "operator / casting")
	})

	Convey("Fragmented text and function-key input can authenticate", t, func() {
		application := newApplication()
		application.Open()

		application.Handle([]byte("oper"))
		application.Handle([]byte("ator\tcast"))
		application.Handle([]byte("ing"))
		So(application.Handle([]byte("\x1b")), ShouldBeEmpty)
		output := string(application.Handle([]byte("OP")))

		So(output, ShouldContainSubstring, "TERMINAL TEST SYSTEM")
		So(output, ShouldContainSubstring, "Authenticated as operator")
		So(output, ShouldNotContainSubstring, "castingcasting")
		So(application.page, ShouldEqual, pageMenu)
	})

	Convey("CSI function keys and unsupported sequences are consumed completely", t, func() {
		application := newApplication()
		application.Open()
		application.Handle([]byte("operator\tcasting"))

		So(application.Handle([]byte("\x1b[")), ShouldBeEmpty)
		output := string(application.Handle([]byte("99~\x1b[11~")))

		So(output, ShouldContainSubstring, "TERMINAL TEST SYSTEM")
		So(string(application.username), ShouldEqual, "operator")
	})

	Convey("Invalid credentials retain diagnostic state without exposing the password", t, func() {
		application := newApplication()
		application.Open()

		output := string(application.Handle([]byte("wrong\tsecret\r")))

		So(output, ShouldContainSubstring, "Invalid user ID or password")
		So(output, ShouldNotContainSubstring, "secret")
		So(application.page, ShouldEqual, pageLogin)
		So(application.password, ShouldBeEmpty)
	})

	Convey("Menu arrows select scenarios and F2 signs out", t, func() {
		application := authenticatedApplication()

		output := string(application.Handle([]byte("\x1b[B\r")))
		So(output, ShouldContainSubstring, "Forms and Input Fields scenario selected")

		output = string(application.Handle([]byte("\x1bOQ")))
		So(output, ShouldContainSubstring, "LOGIN / AUTHENTICATION")
		So(output, ShouldContainSubstring, "Signed out")
		So(application.page, ShouldEqual, pageLogin)
	})

	Convey("Option 12 selects VTTEST without launching from Handle", t, func() {
		application := authenticatedApplication()
		application.selected = 11
		runner := &recordingVTTestRunner{}
		application.SetVTTestRunner(runner)

		output := string(application.Handle([]byte("\r")))

		So(output, ShouldContainSubstring, "Launching VTTEST")
		So(application.launchVTTest, ShouldBeTrue)
		So(runner.calls, ShouldEqual, 0)
	})

	Convey("Run suspends and resumes its screen around an injected VTTEST", t, func() {
		application := newApplication()
		runner := &recordingVTTestRunner{}
		application.SetVTTestRunner(runner)
		input := &fragmentReader{fragments: [][]byte{
			[]byte("operator\tcasting\x1bOP" + strings.Repeat("\x1b[B", 11) + "\r"),
			[]byte("\x03"),
		}}
		var output bytes.Buffer

		err := application.Run(input, &output)

		So(err, ShouldBeNil)
		So(runner.calls, ShouldEqual, 1)
		So(output.String(), ShouldContainSubstring,
			"\x1b[0m\x1b[?25h\x1b[?1049l[VTTEST]\x1b[?1049h")
		So(output.String(), ShouldContainSubstring, "VTTEST completed")
	})

	Convey("A VTTEST launch failure is reported in-app and the screen resumes", t, func() {
		application := newApplication()
		application.SetVTTestRunner(&recordingVTTestRunner{err: fmt.Errorf("not installed")})
		application.page = pageMenu
		application.selected = 11
		input := &fragmentReader{fragments: [][]byte{[]byte("\r"), []byte("\x03")}}
		var output bytes.Buffer

		err := application.Run(input, &output)

		So(err, ShouldBeNil)
		So(output.String(), ShouldContainSubstring, "VTTEST unavailable: not installed")
		So(output.String(), ShouldContainSubstring, "\x1b[?1049l[VTTEST]\x1b[?1049h")
	})

	Convey("A fatal VTTEST terminal lifecycle failure stops the application", t, func() {
		application := newApplication()
		application.SetVTTestRunner(&recordingVTTestRunner{
			err: &FatalVTTestError{Err: fmt.Errorf("raw mode unavailable")},
		})
		application.page = pageMenu
		application.selected = 11
		input := &fragmentReader{fragments: [][]byte{[]byte("\r")}}
		var output bytes.Buffer

		err := application.Run(input, &output)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "raw mode unavailable")
		So(output.String(), ShouldNotContainSubstring, "VTTEST unavailable")
	})

	Convey("Run restores the primary screen at end-of-file", t, func() {
		application := newApplication()
		var output bytes.Buffer

		err := application.Run(strings.NewReader(""), &output)

		So(err, ShouldBeNil)
		So(output.String(), ShouldStartWith, "\x1b[?1049h")
		So(output.String(), ShouldEndWith, "\x1b[0m\x1b[?25h\x1b[?1049l")
	})
}

type recordingVTTestRunner struct {
	calls int
	err   error
}

func (r *recordingVTTestRunner) Run(_ io.Reader, output io.Writer) error {
	r.calls++
	if _, err := output.Write([]byte("[VTTEST]")); err != nil {
		return fmt.Errorf("writing recorded VTTEST output: %w", err)
	}
	return r.err
}

type fragmentReader struct {
	fragments [][]byte
}

func (r *fragmentReader) Read(data []byte) (int, error) {
	if len(r.fragments) == 0 {
		return 0, io.EOF
	}
	fragment := r.fragments[0]
	r.fragments = r.fragments[1:]
	return copy(data, fragment), nil
}

func newApplication() *Application {
	application, err := New(80, 24)
	So(err, ShouldBeNil)
	return application
}

func authenticatedApplication() *Application {
	application := newApplication()
	application.Open()
	application.Handle([]byte("operator\tcasting\x1bOP"))
	So(application.page, ShouldEqual, pageMenu)
	return application
}
