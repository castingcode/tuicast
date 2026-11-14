//go:build !linux && !darwin

package pty

import (
	"fmt"
	"os/exec"
	"runtime"
)

const supported = false

var errUnsupported = fmt.Errorf("PTY sessions are not supported on %s", runtime.GOOS)

// stream is never created on this platform; its methods exist so the
// platform-independent code compiles.
type stream struct{}

func startStream(*exec.Cmd, int, int, func(*stream)) (*stream, error) {
	return nil, errUnsupported
}

func (*stream) Read([]byte) (int, error)  { return 0, errUnsupported }
func (*stream) Write([]byte) (int, error) { return 0, errUnsupported }
func (*stream) Resize(int, int) error     { return errUnsupported }
func (*stream) Close() error              { return errUnsupported }
