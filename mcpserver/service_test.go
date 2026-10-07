package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/mcpserver"
	"github.com/castingcode/tuicast/memory"
	"github.com/castingcode/tuicast/vt220"
	"github.com/castingcode/tuicast/workbench"
	"github.com/google/jsonschema-go/jsonschema"
	. "github.com/smartystreets/goconvey/convey"
)

func TestService(t *testing.T) {
	Convey("A named profile owns a complete terminal automation lifecycle", t, func() {
		service := newService(func(_ context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("READY"))
			input := make([]byte, 4)
			if _, err := io.ReadFull(remote, input); err == nil && string(input) == "abc\r" {
				_, _ = remote.Write([]byte(" ACCEPTED"))
			}
		})
		defer service.Close()

		profiles := service.Profiles()
		So(profiles, ShouldResemble, []mcpserver.ProfileInfo{{
			Name: "warehouse", Description: "test endpoint", Protocol: "memory",
			Terminal: "vt220", Width: 20, Height: 3,
		}})

		connection, err := service.Connect(context.Background(), "warehouse")
		So(err, ShouldBeNil)
		session, err := service.OpenSession(context.Background(), connection.ConnectionID)
		So(err, ShouldBeNil)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		screen, err := service.WaitForText(ctx, session.SessionID, "READY", time.Second)
		So(err, ShouldBeNil)
		So(screen.Text, ShouldStartWith, "READY")

		So(service.Type(session.SessionID, "abc", ""), ShouldBeNil)
		So(service.Press(session.SessionID, tuicast.KeyEnter), ShouldBeNil)
		screen, err = service.WaitForText(ctx, session.SessionID, "ACCEPTED", time.Second)
		So(err, ShouldBeNil)
		So(screen.Text, ShouldContainSubstring, "READY ACCEPTED")

		closed, err := service.CloseSession(session.SessionID)
		So(err, ShouldBeNil)
		So(closed.Closed, ShouldBeTrue)
		closed, err = service.CloseSession(session.SessionID)
		So(err, ShouldBeNil)
		So(closed.Closed, ShouldBeFalse)
		closed, err = service.CloseConnection(connection.ConnectionID)
		So(err, ShouldBeNil)
		So(closed.Closed, ShouldBeTrue)
		So(service.Close(), ShouldBeNil)
		So(service.Close(), ShouldBeNil)
	})

	Convey("Only configured profile names can select a destination", t, func() {
		service := newService(func(context.Context, io.ReadWriteCloser) {})
		defer service.Close()

		connection, err := service.Connect(context.Background(), "arbitrary.example:22")
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "unknown profile")
		So(connection.ConnectionID, ShouldEqual, uint64(0))
	})

	Convey("Wait timeouts are bounded and preserve the last screen for diagnosis", t, func() {
		service := newService(func(_ context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("NOT YET"))
			<-time.After(time.Second)
		})
		defer service.Close()
		connection, err := service.Connect(context.Background(), "warehouse")
		So(err, ShouldBeNil)
		session, err := service.OpenSession(context.Background(), connection.ConnectionID)
		So(err, ShouldBeNil)

		_, err = service.WaitForText(context.Background(), session.SessionID, "READY", 5*time.Millisecond)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "last screen")
		_, err = service.WaitForText(context.Background(), session.SessionID, "READY", 3*time.Minute)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "timeout must be")
	})
}

func TestServiceWaitsAndRecordings(t *testing.T) {
	Convey("Matcher and idle waits are recorded with input as a replayable workflow", t, func() {
		service := newService(func(_ context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("LOGIN\r\nUser: "))
			_, _ = io.Copy(remote, remote)
		})
		defer service.Close()
		connection, err := service.Connect(context.Background(), "warehouse")
		So(err, ShouldBeNil)
		session, err := service.OpenSession(context.Background(), connection.ConnectionID)
		So(err, ShouldBeNil)

		recording, err := service.StartRecording(session.SessionID)
		So(err, ShouldBeNil)
		So(recording.Version, ShouldEqual, workbench.RecordingVersion)
		So(recording.Session, ShouldResemble, workbench.Session{ID: session.SessionID, Terminal: "vt220", Width: 20, Height: 3})
		_, err = service.StartRecording(session.SessionID)
		So(err.Error(), ShouldContainSubstring, "already recording")

		_, err = service.WaitForText(context.Background(), session.SessionID, "LOGIN", 0)
		So(err, ShouldBeNil)
		login := fmt.Sprintf("%-20s", "LOGIN")
		errorText := "ERROR"
		matcher := tuicast.MatcherSpec{All: []tuicast.MatcherSpec{
			{Line: &tuicast.LineMatcherSpec{Row: 0, Text: login}},
			{Cursor: &tuicast.CursorMatcherSpec{Column: 6, Row: 1}},
			{Not: &tuicast.MatcherSpec{Contains: &errorText}},
		}}
		screen, err := service.Wait(context.Background(), session.SessionID, matcher, time.Second, 20*time.Millisecond)
		So(err, ShouldBeNil)
		So(screen.Cursor, ShouldResemble, mcpserver.CursorInfo{Column: 6, Row: 1, Visible: true})

		So(service.Type(session.SessionID, "operator", ""), ShouldBeNil)
		So(service.Press(session.SessionID, tuicast.KeyEnter, tuicast.ModifierShift, tuicast.ModifierShift), ShouldBeNil)
		So(service.Type(session.SessionID, "hunter2", "password"), ShouldBeNil)
		screen, err = service.WaitForIdle(context.Background(), session.SessionID, 20*time.Millisecond, 0)
		So(err, ShouldBeNil)
		So(screen.Text, ShouldContainSubstring, "hunter2")

		never := "NEVER"
		_, err = service.Wait(context.Background(), session.SessionID, tuicast.MatcherSpec{Contains: &never}, 10*time.Millisecond, 0)
		So(err.Error(), ShouldContainSubstring, "last screen")
		_, err = service.Wait(context.Background(), session.SessionID, tuicast.MatcherSpec{}, time.Second, 0)
		So(err.Error(), ShouldContainSubstring, "exactly one matcher expression")
		So(service.Type(session.SessionID, "x", "not valid"), ShouldNotBeNil)

		recording, err = service.StopRecording(session.SessionID)
		So(err, ShouldBeNil)
		So(recording.Steps, ShouldResemble, []workbench.Step{
			{Action: "waitForText", Text: "LOGIN", TimeoutMilliseconds: 10000},
			{Action: "wait", Matcher: &matcher, TimeoutMilliseconds: 1000, StableMilliseconds: 20},
			{Action: "type", Text: "operator"},
			{Action: "press", Key: "Enter", Modifiers: []string{"Shift"}},
			{Action: "type", Parameter: "password"},
			{Action: "waitForIdle", QuietMilliseconds: 20, TimeoutMilliseconds: 10000},
		})
		encoded, err := json.Marshal(recording)
		So(err, ShouldBeNil)
		So(string(encoded), ShouldNotContainSubstring, "hunter2")
		So(string(encoded), ShouldContainSubstring, `"matcher":{"all":[{"line":{"row":0,"text":"LOGIN`)
		So(validateRecording(encoded), ShouldBeNil)
		So(validateRecording([]byte(`{"version":"1","session":{"id":1,"terminal":"vt220","width":20,"height":3},
			"steps":[{"action":"wait","matcher":{"contains":"A","not":{"contains":"B"}},"timeoutMilliseconds":1}]}`)), ShouldNotBeNil)

		_, err = service.StopRecording(session.SessionID)
		So(err.Error(), ShouldContainSubstring, "not recording")
	})

	Convey("Closing a session discards its active recording", t, func() {
		service := newService(func(context.Context, io.ReadWriteCloser) {})
		defer service.Close()
		connection, err := service.Connect(context.Background(), "warehouse")
		So(err, ShouldBeNil)
		session, err := service.OpenSession(context.Background(), connection.ConnectionID)
		So(err, ShouldBeNil)
		_, err = service.StartRecording(session.SessionID)
		So(err, ShouldBeNil)

		_, err = service.CloseSession(session.SessionID)
		So(err, ShouldBeNil)
		_, err = service.StopRecording(session.SessionID)
		So(err.Error(), ShouldContainSubstring, "unknown session")
	})

	Convey("Wait durations are bounded", t, func() {
		service := newService(func(context.Context, io.ReadWriteCloser) {})
		defer service.Close()
		ready := "READY"
		_, err := service.Wait(context.Background(), 1, tuicast.MatcherSpec{Contains: &ready}, time.Second, 3*time.Minute)
		So(err.Error(), ShouldContainSubstring, "stable period must be")
		_, err = service.WaitForIdle(context.Background(), 1, 0, time.Second)
		So(err.Error(), ShouldContainSubstring, "quiet period must be")
		_, err = service.WaitForIdle(context.Background(), 1, time.Millisecond, 3*time.Minute)
		So(err.Error(), ShouldContainSubstring, "timeout must be")
	})
}

// validateRecording checks encoded against the published recording schema.
func validateRecording(encoded []byte) error {
	data, err := os.ReadFile("../schema/workbench-recording.schema.json")
	if err != nil {
		return fmt.Errorf("reading recording schema: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("decoding recording schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolving recording schema: %w", err)
	}
	var instance any
	if err := json.Unmarshal(encoded, &instance); err != nil {
		return fmt.Errorf("decoding recording: %w", err)
	}
	if err := resolved.Validate(instance); err != nil {
		return fmt.Errorf("validating recording: %w", err)
	}
	return nil
}

func newService(handler memory.Handler) *mcpserver.Service {
	connector, err := memory.NewConnector(handler)
	So(err, ShouldBeNil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := mcpserver.New(logger, []mcpserver.Profile{{
		Name: "warehouse", Description: "test endpoint", Protocol: "memory",
		Terminal: tuicast.ProfileVT220, Width: 20, Height: 3, Connector: connector,
	}}, func(profile tuicast.TerminalProfile, width, height int) (tuicast.Terminal, error) {
		return vt220.New(width, height)
	})
	So(err, ShouldBeNil)
	return service
}
