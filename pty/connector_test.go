//go:build linux || darwin

package pty_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/pty"
	"github.com/castingcode/tuicast/vt220"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/term"
)

// helperVariable makes the test binary act as the program under test.
const helperVariable = "TUICAST_PTY_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(helperVariable); mode != "" {
		runHelper(mode, os.Args[1:])
		return
	}
	os.Exit(m.Run())
}

func runHelper(mode string, args []string) {
	switch mode {
	case "interactive":
		width, height, err := term.GetSize(0)
		if err != nil {
			fmt.Printf("SIZE ERROR %v\r\n", err)
			os.Exit(2)
		}
		fmt.Printf("READY TERM=%s SIZE=%dx%d LINES=%s INHERITED=%s GREETING=%s PID=%d\n",
			os.Getenv("TERM"), width, height, os.Getenv("LINES"), os.Getenv("TUICAST_PTY_INHERITED"), os.Getenv("GREETING"), os.Getpid())
		resized := make(chan os.Signal, 1)
		signal.Notify(resized, syscall.SIGWINCH)
		go func() {
			for range resized {
				width, height, _ := term.GetSize(0)
				fmt.Printf("SIZE=%dx%d\n", width, height)
			}
		}()
		lines := bufio.NewScanner(os.Stdin)
		for lines.Scan() {
			fmt.Printf("GOT %s\n", lines.Text())
		}
		os.Exit(0)
	case "exit":
		code, _ := strconv.Atoi(args[0])
		fmt.Printf("BYE %d\n", code)
		os.Exit(code)
	case "flood":
		for line := range 2000 {
			fmt.Printf("line %d\n", line)
		}
		fmt.Print("LAST")
		os.Exit(0)
	case "stubborn":
		signal.Ignore(syscall.SIGHUP)
		fmt.Printf("READY\n")
		time.Sleep(time.Hour)
	case "escape":
		// Start a process outside this process group that keeps the
		// terminal open after this process ends.
		orphan := exec.Command(os.Args[0])
		orphan.Env = append(os.Environ(), helperVariable+"=orphan")
		orphan.Stdin, orphan.Stdout, orphan.Stderr = os.Stdin, os.Stdout, os.Stderr
		orphan.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := orphan.Start(); err != nil {
			fmt.Printf("ORPHAN ERROR %v\n", err)
			os.Exit(2)
		}
		fmt.Printf("ORPHAN=%d\n", orphan.Process.Pid)
		time.Sleep(time.Hour)
	case "orphan":
		signal.Ignore(syscall.SIGHUP)
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(3)
}

func helperConfig(mode string, args ...string) pty.Config {
	return pty.Config{
		Command: os.Args[0],
		Args:    args,
		Env:     []string{helperVariable + "=" + mode},
	}
}

func newTestServer() *tuicast.Server {
	server, err := tuicast.NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	So(err, ShouldBeNil)
	return server
}

func openSession(server *tuicast.Server, config pty.Config) (*tuicast.Connection, *tuicast.Session) {
	connector, err := pty.NewConnector(config)
	So(err, ShouldBeNil)
	connection, err := server.Connect(context.Background(), connector)
	So(err, ShouldBeNil)
	terminal, err := vt220.New(80, 24)
	So(err, ShouldBeNil)
	session, err := connection.NewSession(context.Background(), terminal)
	So(err, ShouldBeNil)
	return connection, session
}

func waitFor(session *tuicast.Session, text string) tuicast.Screen {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	screen, err := session.WaitFor(ctx, tuicast.ScreenContains(text))
	So(err, ShouldBeNil)
	return screen
}

func waitForEnd(session *tuicast.Session) {
	select {
	case <-session.Done():
	case <-time.After(5 * time.Second):
		So("session still active after 5s", ShouldBeEmpty)
	}
}

var readyPID = regexp.MustCompile(`READY .* PID=(\d+)`)

func TestPTYConnector(t *testing.T) {
	Convey("A PTY session runs the program on a terminal of the requested type and size", t, func() {
		server := newTestServer()
		defer server.Close()
		t.Setenv("LINES", "99")
		t.Setenv("TUICAST_PTY_INHERITED", "yes")
		config := helperConfig("interactive")
		config.Env = append(config.Env, "GREETING=hello")
		_, session := openSession(server, config)

		waitFor(session, "READY TERM=vt220 SIZE=80x24 LINES= INHERITED=yes GREETING=hello")

		Convey("input passes through the terminal's line discipline", func() {
			So(session.Send([]byte("pick 42\r")), ShouldBeNil)
			waitFor(session, "GOT pick 42")
		})

		Convey("resizing signals the program with the new size", func() {
			So(session.Resize(100, 40), ShouldBeNil)
			waitFor(session, "SIZE=100x40")
		})

		Convey("ending the program's input ends the session normally", func() {
			So(session.Press(tuicast.Key("d"), tuicast.ModifierControl), ShouldBeNil)
			waitForEnd(session)
			So(session.Err(), ShouldBeNil)
		})
	})

	Convey("A cleared environment contains only the configured variables and TERM", t, func() {
		server := newTestServer()
		defer server.Close()
		t.Setenv("TUICAST_PTY_INHERITED", "yes")
		config := helperConfig("interactive")
		config.ClearEnv = true
		config.Env = append(config.Env, "GREETING=clear")
		_, session := openSession(server, config)

		waitFor(session, "READY TERM=vt220 SIZE=80x24 LINES= INHERITED= GREETING=clear")
	})

	Convey("The program's exit status decides whether the session failed", t, func() {
		server := newTestServer()
		defer server.Close()

		_, succeeded := openSession(server, helperConfig("exit", "0"))
		waitForEnd(succeeded)
		So(succeeded.Err(), ShouldBeNil)
		So(succeeded.Screen().Text(), ShouldContainSubstring, "BYE 0")

		_, failed := openSession(server, helperConfig("exit", "3"))
		waitForEnd(failed)
		So(failed.Err(), ShouldNotBeNil)
		So(failed.Err().Error(), ShouldContainSubstring, "exit status 3")
		So(failed.Screen().Text(), ShouldContainSubstring, "BYE 3")
	})

	Convey("Output written just before the program exits is not lost", t, func() {
		server := newTestServer()
		defer server.Close()
		_, session := openSession(server, helperConfig("flood"))
		waitForEnd(session)
		So(session.Err(), ShouldBeNil)
		So(session.Screen().Text(), ShouldContainSubstring, "line 1999")
		So(session.Screen().Text(), ShouldContainSubstring, "LAST")
	})

	Convey("Each session on a connection runs its own process", t, func() {
		server := newTestServer()
		defer server.Close()
		connection, first := openSession(server, helperConfig("interactive"))
		terminal, err := vt220.New(80, 24)
		So(err, ShouldBeNil)
		second, err := connection.NewSession(context.Background(), terminal)
		So(err, ShouldBeNil)

		firstPID := readyPID.FindStringSubmatch(waitFor(first, "PID=").Text())
		secondPID := readyPID.FindStringSubmatch(waitFor(second, "PID=").Text())
		So(firstPID, ShouldHaveLength, 2)
		So(secondPID, ShouldHaveLength, 2)
		So(firstPID[1], ShouldNotEqual, secondPID[1])

		So(connection.Close(), ShouldBeNil)
		So(processExists(firstPID[1]), ShouldBeFalse)
		So(processExists(secondPID[1]), ShouldBeFalse)
	})

	Convey("Closing a session kills a program that ignores hang-ups", t, func() {
		server := newTestServer()
		defer server.Close()
		_, session := openSession(server, helperConfig("stubborn"))
		waitFor(session, "READY")

		closed := make(chan error, 1)
		go func() { closed <- session.Close() }()
		select {
		case err := <-closed:
			So(err, ShouldBeNil)
		case <-time.After(3 * time.Second):
			So("session close still blocked after 3s", ShouldBeEmpty)
		}
	})

	Convey("Closing a session returns while another process still holds the terminal", t, func() {
		server := newTestServer()
		defer server.Close()
		_, session := openSession(server, helperConfig("escape"))
		screen := waitFor(session, "ORPHAN=")
		orphan := regexp.MustCompile(`ORPHAN=(\d+)`).FindStringSubmatch(screen.Text())
		So(orphan, ShouldHaveLength, 2)
		orphanPID, err := strconv.Atoi(orphan[1])
		So(err, ShouldBeNil)
		defer syscall.Kill(orphanPID, syscall.SIGKILL)

		closed := make(chan error, 1)
		go func() { closed <- session.Close() }()
		select {
		case err := <-closed:
			So(err, ShouldBeNil)
		case <-time.After(3 * time.Second):
			So("session close still blocked after 3s", ShouldBeEmpty)
		}
	})

	Convey("Connect resolves a relative command from the working directory", t, func() {
		server := newTestServer()
		defer server.Close()
		directory := t.TempDir()
		So(os.Symlink(os.Args[0], filepath.Join(directory, "wms-client")), ShouldBeNil)
		config := helperConfig("exit", "0")
		config.Command = "./wms-client"
		config.Dir = directory
		_, session := openSession(server, config)
		waitForEnd(session)
		So(session.Screen().Text(), ShouldContainSubstring, "BYE 0")
	})

	Convey("Configuration errors are reported before a session starts", t, func() {
		_, err := pty.NewConnector(pty.Config{})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "command is required")

		_, err = pty.NewConnector(pty.Config{Command: "sh", Env: []string{"TERM=xterm"}})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "TERM is set from the session's terminal profile")

		_, err = pty.NewConnector(pty.Config{Command: "sh", Env: []string{"NOEQUALS"}})
		So(err, ShouldNotBeNil)

		server := newTestServer()
		defer server.Close()
		connector, err := pty.NewConnector(pty.Config{Command: "tuicast-no-such-program"})
		So(err, ShouldBeNil)
		_, err = server.Connect(context.Background(), connector)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "resolving PTY command")

		connector, err = pty.NewConnector(pty.Config{Command: os.Args[0], Dir: filepath.Join(t.TempDir(), "missing")})
		So(err, ShouldBeNil)
		_, err = server.Connect(context.Background(), connector)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "working directory")
	})
}

func processExists(pid string) bool {
	id, err := strconv.Atoi(pid)
	if err != nil {
		return false
	}
	return syscall.Kill(id, 0) == nil
}
