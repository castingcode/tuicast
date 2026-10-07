package tuicast_test

import (
	"encoding/json"
	"testing"

	"github.com/castingcode/tuicast"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMatcherSpec(t *testing.T) {
	screen := tuicast.Screen{
		Width:  5,
		Height: 2,
		Cells: []tuicast.Cell{
			{Text: "R", Width: 1}, {Text: "E", Width: 1}, {Text: "A", Width: 1}, {Text: "D", Width: 1}, {Text: "Y", Width: 1},
			{Text: ">", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1}, {Text: " ", Width: 1},
		},
		Cursor: tuicast.Cursor{Column: 2, Row: 1},
	}

	Convey("A decoded specification builds an equivalent screen matcher", t, func() {
		var spec tuicast.MatcherSpec
		So(json.Unmarshal([]byte(`{"all":[
			{"contains":"READY"},
			{"line":{"row":1,"text":">    "}},
			{"cursor":{"column":2,"row":1}},
			{"any":[{"contains":"MISSING"},{"not":{"contains":"ERROR"}}]}
		]}`), &spec), ShouldBeNil)

		matcher, err := spec.ScreenMatcher()
		So(err, ShouldBeNil)
		So(matcher.Match(screen), ShouldBeTrue)
		screen.Cursor.Column = 3
		So(matcher.Match(screen), ShouldBeFalse)
	})

	Convey("Invalid specifications are rejected", t, func() {
		text := "READY"
		cases := []struct {
			name    string
			spec    tuicast.MatcherSpec
			message string
		}{
			{"no expression", tuicast.MatcherSpec{}, "exactly one matcher expression"},
			{"two expressions", tuicast.MatcherSpec{Contains: &text, Not: &tuicast.MatcherSpec{Contains: &text}}, "exactly one matcher expression"},
			{"empty all", tuicast.MatcherSpec{All: []tuicast.MatcherSpec{}}, "all requires at least one child"},
			{"empty any", tuicast.MatcherSpec{Any: []tuicast.MatcherSpec{}}, "any requires at least one child"},
		}
		for _, testCase := range cases {
			Convey(testCase.name, func() {
				_, err := testCase.spec.ScreenMatcher()
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, testCase.message)
			})
		}

		Convey("nesting deeper than 64 levels", func() {
			spec := tuicast.MatcherSpec{Contains: &text}
			for range 65 {
				spec = tuicast.MatcherSpec{Not: &spec}
			}
			_, err := spec.ScreenMatcher()
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "nesting exceeds 64 levels")
		})
	})
}
