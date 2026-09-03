package telnet_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	referencetelnet "github.com/castingcode/tuicast/reference/telnet"
	"github.com/castingcode/tuicast/telnet"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
)

func TestServer(t *testing.T) {
	Convey("The reference TUI is served with Telnet negotiation", t, func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		So(err, ShouldBeNil)
		ctx, cancel := context.WithCancel(context.Background())
		serveDone := make(chan error, 1)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		go func() {
			serveDone <- referencetelnet.Serve(ctx, listener, logger)
		}()
		defer func() {
			cancel()
			So(<-serveDone, ShouldBeNil)
		}()

		connector, err := telnet.NewConnector(telnet.Config{Address: listener.Addr().String()})
		So(err, ShouldBeNil)
		server, err := tuicast.NewServer(logger)
		So(err, ShouldBeNil)
		defer func() { So(server.Close(), ShouldBeNil) }()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		terminal, err := vt220.New(80, 24)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), terminal)
		So(err, ShouldBeNil)

		waitContext, stopWaiting := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopWaiting()
		screen, err := session.WaitFor(waitContext, tuicast.ScreenContains("LOGIN / AUTHENTICATION"))
		So(err, ShouldBeNil)
		So(screen.Text(), ShouldContainSubstring, "User ID")
	})
}
