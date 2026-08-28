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
}
