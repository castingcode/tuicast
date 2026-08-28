package telnet_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/telnet"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
)

const (
	se   = 240
	sb   = 250
	will = 251
	do   = 253
	iac  = 255

	ttype = 24
	naws  = 31
)

func TestTelnetConnector(t *testing.T) {
	Convey("Telnet negotiates terminal type and window size while carrying application data", t, func() {
		client, remote := net.Pipe()
		peerErrors := make(chan error, 1)
		go func() {
			defer remote.Close()
			peerErrors <- exerciseTelnetPeer(remote)
		}()

		connector, err := telnet.NewConnector(telnet.Config{
			Address: "in-memory",
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return client, nil
			},
		})
		So(err, ShouldBeNil)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		server, err := tuicast.NewServer(logger)
		So(err, ShouldBeNil)
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		terminal, err := vt220.New(80, 24)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), terminal)
		So(err, ShouldBeNil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err = session.WaitFor(ctx, tuicast.ScreenContains("TELNET READY"))
		So(err, ShouldBeNil)
		So(session.Resize(100, 40), ShouldBeNil)
		So(session.Send([]byte{'A', iac, 'B'}), ShouldBeNil)
		So(<-peerErrors, ShouldBeNil)
		So(server.Close(), ShouldBeNil)
	})
}

func exerciseTelnetPeer(conn net.Conn) error {
	if err := writeAll(conn, []byte{iac, do, ttype}); err != nil {
		return err
	}
	if err := expect(conn, []byte{iac, will, ttype}); err != nil {
		return err
	}
	if err := writeAll(conn, []byte{iac, sb, ttype, 1, iac, se}); err != nil {
		return err
	}
	if err := expect(conn, append([]byte{iac, sb, ttype, 0}, append([]byte("vt220"), iac, se)...)); err != nil {
		return err
	}
	if err := writeAll(conn, []byte{iac, do, naws}); err != nil {
		return err
	}
	if err := expect(conn, []byte{iac, will, naws}); err != nil {
		return err
	}
	if err := expect(conn, []byte{iac, sb, naws, 0, 80, 0, 24, iac, se}); err != nil {
		return err
	}
	if err := writeAll(conn, []byte("\x1b[2JTELNET READY")); err != nil {
		return err
	}
	if err := expect(conn, []byte{iac, sb, naws, 0, 100, 0, 40, iac, se}); err != nil {
		return err
	}
	return expect(conn, []byte{'A', iac, iac, 'B'})
}

func expect(reader io.Reader, expected []byte) error {
	actual := make([]byte, len(expected))
	if _, err := io.ReadFull(reader, actual); err != nil {
		return fmt.Errorf("reading Telnet peer data: %w", err)
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("checking Telnet peer data: got %v, want %v", actual, expected)
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		data = data[n:]
		if err != nil {
			return fmt.Errorf("writing Telnet peer data: %w", err)
		}
	}
	return nil
}
