package schema

import (
	"encoding/json"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestOpenRPCDocument(t *testing.T) {
	Convey("The OpenRPC document describes every callable driver method", t, func() {
		data, err := os.ReadFile("openrpc.json")
		So(err, ShouldBeNil)

		var document struct {
			OpenRPC string `json:"openrpc"`
			Info    struct {
				Title   string `json:"title"`
				Version string `json:"version"`
			} `json:"info"`
			Methods []struct {
				Name           string `json:"name"`
				ParamStructure string `json:"paramStructure"`
			} `json:"methods"`
		}
		So(json.Unmarshal(data, &document), ShouldBeNil)
		So(document.OpenRPC, ShouldEqual, "1.3.2")
		So(document.Info.Title, ShouldEqual, "TUICast Driver API")
		So(document.Info.Version, ShouldEqual, "1")

		names := make([]string, len(document.Methods))
		for index, method := range document.Methods {
			names[index] = method.Name
			So(method.ParamStructure, ShouldEqual, "by-name")
		}
		So(names, ShouldResemble, []string{
			"driver.ping",
			"driver.shutdown",
			"connection.open",
			"connection.close",
			"session.open",
			"session.close",
			"session.send",
			"session.press",
			"session.resize",
			"session.screen",
			"session.wait",
			"session.waitForIdle",
			"session.subscribe",
			"session.subscribeEvents",
			"session.unsubscribe",
		})
	})
}
