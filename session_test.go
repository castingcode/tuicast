package tuicast_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/memory"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSessionVerticalSlice(t *testing.T) {
	Convey("Host output, input, waits, and resizing flow through one session", t, func() {
		input := make(chan string, 1)
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("\x1b[2;"))
			_, _ = remote.Write([]byte("3HREADY"))
			buffer := make([]byte, 16)
			n, readErr := remote.Read(buffer)
			if readErr == nil {
				input <- string(buffer[:n])
			}
			<-ctx.Done()
		})
		So(err, ShouldBeNil)

		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		terminal := newTestTerminal(12, 4)
		session, err := connection.NewSession(context.Background(), terminal)
		So(err, ShouldBeNil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		screen, err := session.WaitFor(ctx, tuicast.ScreenContains("READY"))
		So(err, ShouldBeNil)
		So(screen.Line(1), ShouldEqual, "  READY     ")

		So(session.Send([]byte("pick-42")), ShouldBeNil)
		So(<-input, ShouldEqual, "pick-42")
		So(session.Resize(20, 6), ShouldBeNil)
		So(session.Screen().Width, ShouldEqual, 20)
		So(session.Screen().Height, ShouldEqual, 6)
		So(server.Close(), ShouldBeNil)
	})

	Convey("WaitFor respects context cancellation", t, func() {
		connector, err := memory.NewConnector(func(ctx context.Context, _ io.ReadWriteCloser) {
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), newTestTerminal(10, 2))
		So(err, ShouldBeNil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = session.WaitFor(ctx, tuicast.ScreenContains("never"))
		So(err, ShouldNotBeNil)
		var waitErr *tuicast.WaitError
		So(errors.As(err, &waitErr), ShouldBeTrue)
		So(errors.Is(err, context.Canceled), ShouldBeTrue)
		So(waitErr.Expected, ShouldEqual, `screen containing "never"`)
		So(strings.Contains(err.Error(), "last screen:"), ShouldBeTrue)
		So(server.Close(), ShouldBeNil)
	})
}

func TestSessionAutomation(t *testing.T) {
	Convey("WaitForStable waits through visible and invisible host output", t, func() {
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("READY"))
			time.Sleep(30 * time.Millisecond)
			_, _ = remote.Write([]byte{0})
			time.Sleep(30 * time.Millisecond)
			_, _ = remote.Write([]byte(" REST"))
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), newTestTerminal(16, 2))
		So(err, ShouldBeNil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		started := time.Now()
		screen, err := session.WaitForStable(ctx, tuicast.ScreenContains("READY"), 50*time.Millisecond)
		So(err, ShouldBeNil)
		So(screen.Line(0), ShouldStartWith, "READY REST")
		So(time.Since(started), ShouldBeGreaterThanOrEqualTo, 90*time.Millisecond)
		So(server.Close(), ShouldBeNil)
	})

	Convey("WaitForIdle resets its quiet period for all host output", t, func() {
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			_, _ = remote.Write([]byte("READY"))
			time.Sleep(30 * time.Millisecond)
			_, _ = remote.Write([]byte{0})
			time.Sleep(30 * time.Millisecond)
			_, _ = remote.Write([]byte(" DONE"))
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), newTestTerminal(16, 2))
		So(err, ShouldBeNil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		started := time.Now()
		screen, err := session.WaitForIdle(ctx, 50*time.Millisecond)
		So(err, ShouldBeNil)
		So(screen.Line(0), ShouldStartWith, "READY DONE")
		So(time.Since(started), ShouldBeGreaterThanOrEqualTo, 90*time.Millisecond)
		So(server.Close(), ShouldBeNil)
	})

	Convey("Press sends the terminal profile's named-key sequence", t, func() {
		input := make(chan string, 1)
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			buffer := make([]byte, 3)
			_, readErr := io.ReadFull(remote, buffer)
			if readErr == nil {
				input <- string(buffer)
			}
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), newTestTerminal(10, 2))
		So(err, ShouldBeNil)

		So(session.Press(tuicast.KeyUp), ShouldBeNil)
		So(<-input, ShouldEqual, "\x1b[A")
		So(session.Press(tuicast.Key("unsupported")), ShouldNotBeNil)
		So(server.Close(), ShouldBeNil)
	})

	Convey("Press combines modifiers for printable keys", t, func() {
		input := make(chan byte, 1)
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			buffer := make([]byte, 1)
			_, readErr := io.ReadFull(remote, buffer)
			if readErr == nil {
				input <- buffer[0]
			}
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), newTestTerminal(10, 2))
		So(err, ShouldBeNil)

		So(session.Press(tuicast.Key("c"), tuicast.ModifierControl), ShouldBeNil)
		So(<-input, ShouldEqual, byte(0x03))
		So(server.Close(), ShouldBeNil)
	})

	Convey("BELL is observable and ENQ sends the configured answerback", t, func() {
		start := make(chan struct{})
		answerback := make(chan string, 1)
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			select {
			case <-start:
			case <-ctx.Done():
				return
			}
			_, _ = remote.Write([]byte{'\a', 0x05})
			buffer := make([]byte, len("TUICAST"))
			if _, readErr := io.ReadFull(remote, buffer); readErr == nil {
				answerback <- string(buffer)
			}
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		terminal := newTestTerminal(10, 2)
		So(terminal.SetAnswerback("TUICAST"), ShouldBeNil)
		session, err := connection.NewSession(context.Background(), terminal)
		So(err, ShouldBeNil)
		events := session.Events(context.Background())
		close(start)

		bell := <-events
		enquiry := <-events
		So(bell, ShouldResemble, tuicast.TerminalEvent{Sequence: 1, Type: tuicast.EventBell})
		So(enquiry, ShouldResemble, tuicast.TerminalEvent{Sequence: 2, Type: tuicast.EventEnquiry, Data: "TUICAST"})
		So(<-answerback, ShouldEqual, "TUICAST")
		So(server.Close(), ShouldBeNil)
	})

	Convey("Screens emits the initial and changed snapshots and closes on cancellation", t, func() {
		write := make(chan struct{})
		connector, err := memory.NewConnector(func(ctx context.Context, remote io.ReadWriteCloser) {
			select {
			case <-write:
				_, _ = remote.Write([]byte("UPDATED"))
			case <-ctx.Done():
				return
			}
			<-ctx.Done()
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)
		session, err := connection.NewSession(context.Background(), newTestTerminal(10, 2))
		So(err, ShouldBeNil)

		ctx, cancel := context.WithCancel(context.Background())
		screens := session.Screens(ctx)
		So((<-screens).Revision, ShouldEqual, uint64(0))
		close(write)
		So((<-screens).Line(0), ShouldStartWith, "UPDATED")
		cancel()
		_, open := <-screens
		So(open, ShouldBeFalse)
		So(server.Close(), ShouldBeNil)
	})
}

func TestServerConcurrency(t *testing.T) {
	Convey("Many sessions remain isolated and close with their server", t, func() {
		connector, err := memory.NewConnector(func(_ context.Context, remote io.ReadWriteCloser) {
			_, _ = io.Copy(remote, remote)
		})
		So(err, ShouldBeNil)
		server := newTestServer()
		connection, err := server.Connect(context.Background(), connector)
		So(err, ShouldBeNil)

		const count = 100
		sessions := make([]*tuicast.Session, count)
		var waitGroup sync.WaitGroup
		errorsFound := make(chan error, count)
		for i := range count {
			waitGroup.Add(1)
			go func(index int) {
				defer waitGroup.Done()
				terminal, terminalErr := vt220.New(16, 2)
				if terminalErr != nil {
					errorsFound <- terminalErr
					return
				}
				session, openErr := connection.NewSession(context.Background(), terminal)
				if openErr != nil {
					errorsFound <- openErr
					return
				}
				sessions[index] = session
				label := []byte(fmt.Sprintf("session-%03d", index))
				if sendErr := session.Send(label); sendErr != nil {
					errorsFound <- sendErr
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if _, waitErr := session.WaitFor(ctx, tuicast.ScreenContains(string(label))); waitErr != nil {
					errorsFound <- waitErr
				}
			}(i)
		}
		waitGroup.Wait()
		close(errorsFound)
		So(len(errorsFound), ShouldEqual, 0)

		closeErrors := make(chan error, 8)
		for range 8 {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				closeErrors <- server.Close()
			}()
		}
		waitGroup.Wait()
		close(closeErrors)
		for closeErr := range closeErrors {
			So(closeErr, ShouldBeNil)
		}
		for _, session := range sessions {
			select {
			case <-session.Done():
			default:
				So("session still running", ShouldBeBlank)
			}
		}
	})
}

func newTestServer() *tuicast.Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := tuicast.NewServer(logger)
	So(err, ShouldBeNil)
	return server
}

func newTestTerminal(width, height int) *vt220.Terminal {
	terminal, err := vt220.New(width, height)
	So(err, ShouldBeNil)
	return terminal
}
