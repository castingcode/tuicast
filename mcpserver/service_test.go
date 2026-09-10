package mcpserver_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/mcpserver"
	"github.com/castingcode/tuicast/memory"
	"github.com/castingcode/tuicast/vt220"
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

		So(service.Type(session.SessionID, "abc"), ShouldBeNil)
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
