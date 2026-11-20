//go:build linux || darwin

package pty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/castingcode/tuicast"
	creackpty "github.com/creack/pty"
	"golang.org/x/sys/unix"
)

var _ tuicast.SessionStream = (*stream)(nil)

const supported = true

var errUnsupported = errors.New("PTY sessions are not supported on this platform")

// killGrace is how long Close waits after hanging up the program before it
// kills the program's process group.
const killGrace = time.Second

// stream is one program running on a PTY. Reads and writes use the PTY's
// controlling side.
type stream struct {
	master  *os.File
	path    string
	process *os.Process
	onClose func(*stream)

	exited  chan struct{}
	waitErr error // written before exited closes

	closing   chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func startStream(cmd *exec.Cmd, width, height int, onClose func(*stream)) (*stream, error) {
	master, tty, err := creackpty.Open()
	if err != nil {
		return nil, fmt.Errorf("opening PTY: %w", err)
	}
	defer tty.Close()
	if err := creackpty.Setsize(master, &creackpty.Winsize{Cols: uint16(width), Rows: uint16(height)}); err != nil {
		return nil, errors.Join(fmt.Errorf("setting PTY size: %w", err), master.Close())
	}
	// Close can only interrupt a pending Read when the runtime poller owns
	// the descriptor. creack/pty opens the macOS master in blocking mode,
	// and os.File.Fd switches any file to blocking mode, so replace the file
	// with a non-blocking duplicate.
	pollable, err := pollableFile(master)
	if err != nil {
		return nil, err
	}

	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	// Make the program a session leader with the PTY as its controlling
	// terminal. If TUICast exits without closing the session, the kernel
	// hangs up the terminal and signals the program.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(err, pollable.Close())
	}

	opened := &stream{
		master:  pollable,
		path:    cmd.Path,
		process: cmd.Process,
		onClose: onClose,
		exited:  make(chan struct{}),
		closing: make(chan struct{}),
	}
	go func() {
		opened.waitErr = cmd.Wait()
		close(opened.exited)
	}()
	return opened, nil
}

func pollableFile(master *os.File) (*os.File, error) {
	duplicate, err := syscall.Dup(int(master.Fd()))
	closeErr := master.Close()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("duplicating PTY descriptor: %w", err), closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(fmt.Errorf("closing original PTY descriptor: %w", closeErr), syscall.Close(duplicate))
	}
	syscall.CloseOnExec(duplicate)
	if err := syscall.SetNonblock(duplicate, true); err != nil {
		return nil, errors.Join(fmt.Errorf("making PTY descriptor non-blocking: %w", err), syscall.Close(duplicate))
	}
	return os.NewFile(uintptr(duplicate), "/dev/ptmx"), nil
}

// Read returns the program's output. Once the program has closed the
// terminal, Read waits for it to exit and returns io.EOF for a zero exit
// status, or an error describing how it exited.
func (s *stream) Read(data []byte) (int, error) {
	n, err := s.master.Read(data)
	if err == nil || n > 0 {
		return n, nil
	}
	// Linux reports EIO and macOS reports EOF when no process has the
	// terminal open any longer.
	if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
		return 0, fmt.Errorf("reading PTY: %w", err)
	}
	select {
	case <-s.exited:
	case <-s.closing:
		return 0, io.EOF
	}
	if s.waitErr != nil {
		return 0, fmt.Errorf("running %s: %w", s.path, s.waitErr)
	}
	return 0, io.EOF
}

func (s *stream) Write(data []byte) (int, error) {
	n, err := s.master.Write(data)
	if err != nil {
		return n, fmt.Errorf("writing PTY: %w", err)
	}
	return n, nil
}

// Resize sets the PTY window size, which signals the program with SIGWINCH.
func (s *stream) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("resizing PTY: dimensions must be positive")
	}
	if width > maximumDimension || height > maximumDimension {
		return fmt.Errorf("resizing PTY: dimensions must not exceed %d", maximumDimension)
	}
	connection, err := s.master.SyscallConn()
	if err != nil {
		return fmt.Errorf("resizing PTY: %w", err)
	}
	var ioctlErr error
	if err := connection.Control(func(descriptor uintptr) {
		ioctlErr = unix.IoctlSetWinsize(int(descriptor), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(width), Row: uint16(height)})
	}); err != nil {
		return fmt.Errorf("resizing PTY: %w", err)
	}
	if ioctlErr != nil {
		return fmt.Errorf("resizing PTY: %w", ioctlErr)
	}
	return nil
}

// Close hangs up the program's process group, kills the group if the
// program is still running after a grace period, and reaps the program.
// Closing the PTY unblocks any pending Read.
func (s *stream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closing)
		var closeErrors []error
		if err := s.signalGroup(syscall.SIGHUP); err != nil {
			closeErrors = append(closeErrors, err)
		}
		if err := s.master.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			closeErrors = append(closeErrors, fmt.Errorf("closing PTY: %w", err))
		}
		select {
		case <-s.exited:
		case <-time.After(killGrace):
			if err := s.signalGroup(syscall.SIGKILL); err != nil {
				closeErrors = append(closeErrors, err)
			}
			<-s.exited
		}
		s.closeErr = errors.Join(closeErrors...)
		if s.onClose != nil {
			s.onClose(s)
		}
	})
	return s.closeErr
}

// signalGroup signals the program's process group, whose ID is the program's
// process ID because the program is a session leader. It does nothing once
// the program has been reaped, because its ID may then be reused.
func (s *stream) signalGroup(signal syscall.Signal) error {
	select {
	case <-s.exited:
		return nil
	default:
	}
	if err := syscall.Kill(-s.process.Pid, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("sending %s to PTY process group: %w", signal, err)
	}
	return nil
}
