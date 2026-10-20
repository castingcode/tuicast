// Package trace encodes TUICast flight recordings as JSON-lines trace files.
//
// A dump is one JSON object per line: a header, the recorded output and
// resize records in order, one record per unsupported sequence, and a final
// record describing the wait failure. Output bytes are base64 so arbitrary
// bytes, including invalid UTF-8, survive exactly. See
// design/troubleshooting.md for the format.
package trace

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/castingcode/tuicast"
)

// Version identifies the dump format written by WriteDump.
const Version = 1

type header struct {
	Type         string `json:"type"`
	Version      int    `json:"version"`
	Terminal     string `json:"terminal"`
	Start        string `json:"start"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	DroppedBytes uint64 `json:"droppedBytes"`
}

type output struct {
	Type               string `json:"type"`
	OffsetMicroseconds int64  `json:"offsetMicroseconds"`
	Revision           uint64 `json:"revision"`
	Base64             []byte `json:"base64"`
}

type resize struct {
	Type               string `json:"type"`
	OffsetMicroseconds int64  `json:"offsetMicroseconds"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
}

type unsupported struct {
	Type     string `json:"type"`
	Sequence string `json:"sequence"`
	Count    uint64 `json:"count"`
}

type waitFailure struct {
	Type               string `json:"type"`
	OffsetMicroseconds int64  `json:"offsetMicroseconds"`
	Expected           string `json:"expected"`
	Cause              string `json:"cause"`
	Screen             screen `json:"screen"`
}

type screen struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Revision uint64 `json:"revision"`
	Cursor   cursor `json:"cursor"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text"`
}

type cursor struct {
	Column  int  `json:"column"`
	Row     int  `json:"row"`
	Visible bool `json:"visible"`
}

// WriteDump writes failure's flight recording and last screen as a JSON-lines
// dump.
func WriteDump(w io.Writer, failure *tuicast.WaitError) error {
	if failure.Recording == nil {
		return fmt.Errorf("writing trace dump: wait error has no flight recording")
	}
	recording := failure.Recording
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	write := func(record any) error {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("writing trace dump: %w", err)
		}
		return nil
	}

	if err := write(header{
		Type:         "header",
		Version:      Version,
		Terminal:     string(recording.Profile),
		Start:        recording.Start.UTC().Format(time.RFC3339Nano),
		Width:        recording.Width,
		Height:       recording.Height,
		DroppedBytes: recording.DroppedBytes,
	}); err != nil {
		return err
	}
	for _, record := range recording.Records {
		var entry any = resize{
			Type:               "resize",
			OffsetMicroseconds: record.Offset.Microseconds(),
			Width:              record.Width,
			Height:             record.Height,
		}
		if record.Type == tuicast.FlightOutput {
			entry = output{
				Type:               "output",
				OffsetMicroseconds: record.Offset.Microseconds(),
				Revision:           record.Revision,
				Base64:             record.Data,
			}
		}
		if err := write(entry); err != nil {
			return err
		}
	}
	for _, sequence := range recording.Unsupported {
		if err := write(unsupported{Type: "unsupported", Sequence: sequence.Sequence, Count: sequence.Count}); err != nil {
			return err
		}
	}
	last := failure.LastScreen
	cause := ""
	if failure.Cause != nil {
		cause = failure.Cause.Error()
	}
	return write(waitFailure{
		Type:               "waitFailure",
		OffsetMicroseconds: recording.Captured.Microseconds(),
		Expected:           failure.Expected,
		Cause:              cause,
		Screen: screen{
			Width:    last.Width,
			Height:   last.Height,
			Revision: last.Revision,
			Cursor:   cursor{Column: last.Cursor.Column, Row: last.Cursor.Row, Visible: last.Cursor.Visible},
			Title:    last.Title,
			Text:     last.Text(),
		},
	})
}
