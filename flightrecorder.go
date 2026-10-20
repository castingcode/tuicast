package tuicast

import (
	"sync"
	"time"
)

// Flight recorder bounds. The recorder keeps the most recent output rather
// than a time window, so a wait on a screen that stopped changing long ago
// still captures the bytes that drew it.
const (
	flightRecorderBytes   = 64 * 1024
	flightRecorderRecords = 4096
)

// FlightRecordType identifies an entry in a flight recording.
type FlightRecordType string

const (
	// FlightOutput is one read of host output, recorded exactly as received.
	FlightOutput FlightRecordType = "output"
	// FlightResize is a change of terminal dimensions.
	FlightResize FlightRecordType = "resize"
)

// FlightRecord is one entry in a flight recording. Offset is measured from
// the session start. Output records carry Data and the screen Revision after
// the terminal applied it; resize records carry Width and Height.
type FlightRecord struct {
	Type     FlightRecordType
	Offset   time.Duration
	Data     []byte
	Revision uint64
	Width    int
	Height   int
}

// FlightRecording is a detached copy of a session's most recent host output,
// up to 64 KiB, kept so a failed wait can be traced back to the bytes that
// produced its last screen. Width and Height are the dimensions in effect
// before the first record. DroppedBytes counts earlier output no longer
// retained, so a replay of a recording with dropped bytes starts mid-stream.
type FlightRecording struct {
	Profile      TerminalProfile
	Start        time.Time
	Captured     time.Duration
	Width        int
	Height       int
	DroppedBytes uint64
	Records      []FlightRecord
	Unsupported  []UnsupportedSequence
}

// revisionReporter is implemented by terminals that can report their screen
// revision without copying the screen.
type revisionReporter interface {
	Revision() uint64
}

type flightRecorder struct {
	mu           sync.Mutex
	start        time.Time
	width        int
	height       int
	droppedBytes uint64
	bytes        int
	records      []FlightRecord
}

func newFlightRecorder(start time.Time, width, height int) *flightRecorder {
	return &flightRecorder{start: start, width: width, height: height}
}

// output records data while apply hands it to the terminal and returns the
// resulting screen revision. Holding the lock across apply means any screen a
// caller has observed is already recorded when it takes a recording.
func (r *flightRecorder) output(at time.Time, data []byte, apply func() uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	revision := apply()
	r.records = append(r.records, FlightRecord{
		Type:     FlightOutput,
		Offset:   at.Sub(r.start),
		Data:     append([]byte(nil), data...),
		Revision: revision,
	})
	r.bytes += len(data)
	r.evict()
}

func (r *flightRecorder) resize(at time.Time, width, height int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, FlightRecord{Type: FlightResize, Offset: at.Sub(r.start), Width: width, Height: height})
	r.evict()
}

// evict drops the oldest records until the recording fits its bounds,
// carrying evicted resizes into the starting dimensions.
func (r *flightRecorder) evict() {
	drop := 0
	for drop < len(r.records) && (r.bytes > flightRecorderBytes || len(r.records)-drop > flightRecorderRecords) {
		record := r.records[drop]
		if record.Type == FlightResize {
			r.width, r.height = record.Width, record.Height
		} else {
			r.bytes -= len(record.Data)
			r.droppedBytes += uint64(len(record.Data))
		}
		drop++
	}
	r.records = r.records[drop:]
}

func (r *flightRecorder) recording(profile TerminalProfile, at time.Time, unsupported []UnsupportedSequence) FlightRecording {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]FlightRecord, len(r.records))
	for index, record := range r.records {
		record.Data = append([]byte(nil), record.Data...)
		records[index] = record
	}
	return FlightRecording{
		Profile:      profile,
		Start:        r.start,
		Captured:     at.Sub(r.start),
		Width:        r.width,
		Height:       r.height,
		DroppedBytes: r.droppedBytes,
		Records:      records,
		Unsupported:  unsupported,
	}
}
