package parsing_test

import (
	"testing"

	"github.com/castingcode/tuicast/parsing"
	. "github.com/smartystreets/goconvey/convey"
)

func TestParser(t *testing.T) {
	Convey("The parser preserves state across fragmented writes", t, func() {
		var printed []rune
		var command parsing.Command
		var parameters []parsing.Parameter
		parser := parsing.New(parsing.Handler{
			Print: func(r rune) {
				printed = append(printed, r)
			},
			CSI: func(parsed parsing.Command, parsedParameters []parsing.Parameter) {
				command = parsed
				parameters = parsedParameters
			},
		})

		_, err := parser.Write([]byte("A\x1b[2;"))
		So(err, ShouldBeNil)
		So(string(printed), ShouldEqual, "A")
		So(command.Final, ShouldEqual, byte(0))

		_, err = parser.Write([]byte("3HB"))
		So(err, ShouldBeNil)
		So(string(printed), ShouldEqual, "AB")
		So(command.Final, ShouldEqual, byte('H'))
		So(parameters, ShouldResemble, []parsing.Parameter{
			{Value: 2, Present: true},
			{Value: 3, Present: true},
		})
	})

	Convey("The parser distinguishes omitted parameters", t, func() {
		var parameters []parsing.Parameter
		parser := parsing.New(parsing.Handler{
			CSI: func(_ parsing.Command, parsed []parsing.Parameter) {
				parameters = parsed
			},
		})

		_, err := parser.Write([]byte("\x1b[;4H"))
		So(err, ShouldBeNil)
		So(parameters, ShouldResemble, []parsing.Parameter{
			{Present: false},
			{Value: 4, Present: true},
		})
	})

	Convey("The parser dispatches control strings without printing them", t, func() {
		type osc struct {
			command int
			data    string
		}
		var printed []rune
		var executed []byte
		var oscs []osc
		var dcsCommand parsing.Command
		var dcsParameters []parsing.Parameter
		var dcsData string
		var controlStrings []string
		parser := parsing.New(parsing.Handler{
			Print:   func(r rune) { printed = append(printed, r) },
			Execute: func(b byte) { executed = append(executed, b) },
			OSC: func(command int, data []byte) {
				oscs = append(oscs, osc{command, string(data)})
			},
			DCS: func(command parsing.Command, parameters []parsing.Parameter, data []byte) {
				dcsCommand, dcsParameters, dcsData = command, parameters, string(data)
			},
			ControlString: func(kind string) {
				controlStrings = append(controlStrings, kind)
			},
		})

		_, err := parser.Write([]byte("A\x1b]0;RF-01\x07\x1b]52;c;aGk=\x1b\\\x1b]1\x07\x1b]title\x07B"))
		So(err, ShouldBeNil)
		_, err = parser.Write([]byte("\x1bP0;1|17/41\x1b\\\x1b^pm\x1b\\\x1b_apc\x1b\\\x1bXsos\x1b\\C"))
		So(err, ShouldBeNil)

		So(string(printed), ShouldEqual, "ABC")
		So(executed, ShouldBeEmpty)
		So(oscs, ShouldResemble, []osc{{0, "RF-01"}, {52, "c;aGk="}, {1, ""}, {-1, "title"}})
		So(dcsCommand, ShouldResemble, parsing.Command{Final: '|'})
		So(dcsParameters, ShouldResemble, []parsing.Parameter{{Value: 0, Present: true}, {Value: 1, Present: true}})
		So(dcsData, ShouldEqual, "17/41")
		So(controlStrings, ShouldResemble, []string{"PM", "APC", "SOS"})
	})

	Convey("UTF-8 inside control strings does not end them early", t, func() {
		var printed []rune
		var executed []byte
		var titles []string
		var dcsData string
		var controlStrings []string
		parser := parsing.New(parsing.Handler{
			Print:   func(r rune) { printed = append(printed, r) },
			Execute: func(b byte) { executed = append(executed, b) },
			OSC:     func(_ int, data []byte) { titles = append(titles, string(data)) },
			DCS:     func(_ parsing.Command, _ []parsing.Parameter, data []byte) { dcsData = string(data) },
			ControlString: func(kind string) {
				controlStrings = append(controlStrings, kind)
			},
		})

		// “ and 本 each contain the byte 0x9C, an 8-bit string terminator.
		_, err := parser.Write([]byte("\x1b]2;“Pick” 本\x07A\x1bP1$q本\x1b\\\x1b_é本\x1b\\\x1b^ü\x1b\\\x1bX日本\x1b\\B"))
		So(err, ShouldBeNil)
		_, err = parser.Write([]byte("\x1b]2;x\x9cC"))
		So(err, ShouldBeNil)

		So(string(printed), ShouldEqual, "ABC")
		So(executed, ShouldBeEmpty)
		So(titles, ShouldResemble, []string{"“Pick” 本", "x"})
		So(dcsData, ShouldEqual, "本")
		So(controlStrings, ShouldResemble, []string{"APC", "PM", "SOS"})
	})
}
