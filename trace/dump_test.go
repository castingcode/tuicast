package trace_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/trace"
	. "github.com/smartystreets/goconvey/convey"
)

func TestWriteDump(t *testing.T) {
	Convey("A dump preserves recorded bytes, order, and the failure", t, func() {
		start := time.Date(2026, 10, 20, 15, 40, 53, 726000000, time.FixedZone("EDT", -4*60*60))
		raw := []byte("\x1b[?1049h\xff\xfeOK")
		failure := &tuicast.WaitError{
			Expected: `screen containing "READY"`,
			Cause:    context.DeadlineExceeded,
			LastScreen: tuicast.Screen{
				Width: 2, Height: 1, Revision: 4, Title: "RF-01",
				Cells:  []tuicast.Cell{{Text: "O", Width: 1}, {Text: "K", Width: 1}},
				Cursor: tuicast.Cursor{Column: 1, Visible: true},
			},
			Recording: &tuicast.FlightRecording{
				Profile: tuicast.ProfileXTerm, Start: start, Captured: 2500 * time.Millisecond,
				Width: 80, Height: 24, DroppedBytes: 12,
				Records: []tuicast.FlightRecord{
					{Type: tuicast.FlightOutput, Offset: 1520 * time.Microsecond, Data: raw, Revision: 3},
					{Type: tuicast.FlightResize, Offset: 9811 * time.Microsecond, Width: 132, Height: 24},
				},
				Unsupported: []tuicast.UnsupportedSequence{{Sequence: "CSI ? 1049 h", Count: 2}},
			},
		}

		var dump bytes.Buffer
		So(trace.WriteDump(&dump, failure), ShouldBeNil)
		var records []map[string]any
		scanner := bufio.NewScanner(&dump)
		for scanner.Scan() {
			var record map[string]any
			So(json.Unmarshal(scanner.Bytes(), &record), ShouldBeNil)
			records = append(records, record)
		}
		So(records, ShouldHaveLength, 5)

		So(records[0], ShouldResemble, map[string]any{
			"type": "header", "version": 1.0, "terminal": "xterm-256color",
			"start": "2026-10-20T19:40:53.726Z", "width": 80.0, "height": 24.0, "droppedBytes": 12.0,
		})
		So(records[1]["type"], ShouldEqual, "output")
		So(records[1]["offsetMicroseconds"], ShouldEqual, 1520)
		So(records[1]["revision"], ShouldEqual, 3)
		decoded, err := base64.StdEncoding.DecodeString(records[1]["base64"].(string))
		So(err, ShouldBeNil)
		So(decoded, ShouldResemble, raw)
		So(records[2], ShouldResemble, map[string]any{
			"type": "resize", "offsetMicroseconds": 9811.0, "width": 132.0, "height": 24.0,
		})
		So(records[3], ShouldResemble, map[string]any{"type": "unsupported", "sequence": "CSI ? 1049 h", "count": 2.0})
		So(records[4]["type"], ShouldEqual, "waitFailure")
		So(records[4]["offsetMicroseconds"], ShouldEqual, 2500000)
		So(records[4]["expected"], ShouldEqual, `screen containing "READY"`)
		So(records[4]["cause"], ShouldEqual, "context deadline exceeded")
		So(records[4]["screen"], ShouldResemble, map[string]any{
			"width": 2.0, "height": 1.0, "revision": 4.0, "title": "RF-01", "text": "OK",
			"cursor": map[string]any{"column": 1.0, "row": 0.0, "visible": true},
		})
	})

	Convey("A wait error without a recording is rejected", t, func() {
		var dump bytes.Buffer
		So(trace.WriteDump(&dump, &tuicast.WaitError{Expected: "x"}), ShouldNotBeNil)
		So(dump.Len(), ShouldEqual, 0)
	})
}
