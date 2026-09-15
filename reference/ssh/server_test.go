package ssh_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	referencessh "github.com/castingcode/tuicast/reference/ssh"
	tuicastssh "github.com/castingcode/tuicast/ssh"
	"github.com/castingcode/tuicast/xterm"
	. "github.com/smartystreets/goconvey/convey"
	gossh "golang.org/x/crypto/ssh"
)

func TestServer(t *testing.T) {
	Convey("The reference TUI is served over authenticated SSH", t, func() {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		So(err, ShouldBeNil)
		signer, err := gossh.NewSignerFromKey(privateKey)
		So(err, ShouldBeNil)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		So(err, ShouldBeNil)
		ctx, cancel := context.WithCancel(context.Background())
		serveDone := make(chan error, 1)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		go func() {
			serveDone <- referencessh.Serve(ctx, listener, referencessh.Config{
				Users:  map[string]string{"operator": "casting", "supervisor": "warehouse"},
				Signer: signer,
				Logger: logger,
			})
		}()
		defer func() {
			cancel()
			So(<-serveDone, ShouldBeNil)
		}()

		connector, err := tuicastssh.NewConnector(tuicastssh.Config{
			Address: listener.Addr().String(),
			ClientConfig: &gossh.ClientConfig{
				User:            "supervisor",
				Auth:            []gossh.AuthMethod{gossh.Password("warehouse")},
				HostKeyCallback: gossh.FixedHostKey(signer.PublicKey()),
			},
		})
		So(err, ShouldBeNil)
		server, err := tuicast.NewServer(logger)
		So(err, ShouldBeNil)
		defer func() { So(server.Close(), ShouldBeNil) }()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		terminal, err := xterm.New(80, 24)
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
