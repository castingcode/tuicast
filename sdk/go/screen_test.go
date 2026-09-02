package tuicast

import (
	"encoding/json"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestScreenQueries(t *testing.T) {
	Convey("Screen queries follow the shared language-neutral fixtures", t, func() {
		data, err := os.ReadFile("../../schema/testdata/screen-queries.json")
		So(err, ShouldBeNil)
		var fixtures struct {
			Cases []struct {
				Name     string   `json:"name"`
				Screen   Screen   `json:"screen"`
				Contains string   `json:"contains"`
				Find     string   `json:"find"`
				Position Position `json:"position"`
			} `json:"cases"`
		}
		So(json.Unmarshal(data, &fixtures), ShouldBeNil)

		for _, fixture := range fixtures.Cases {
			Convey(fixture.Name, func() {
				So(fixture.Screen.Contains(fixture.Contains), ShouldBeTrue)
				position, found := fixture.Screen.Find(fixture.Find)
				So(found, ShouldBeTrue)
				So(position, ShouldResemble, fixture.Position)
				So(fixture.Screen.Line(fixture.Position.Row), ShouldNotBeBlank)
				_, found = fixture.Screen.Find("MISSING")
				So(found, ShouldBeFalse)
			})
		}
	})
}
