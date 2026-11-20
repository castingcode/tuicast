// Package pty runs local programs on pseudo-terminals, so TUICast can automate
// terminal applications without a network service.
package pty

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/castingcode/tuicast"
)

var _ tuicast.Connector = (*Connector)(nil)
var _ tuicast.SessionOpener = (*connection)(nil)

// Config describes the program each session runs.
type Config struct {
	// Command is the program to run. A name without a path separator is
	// looked up on the PATH of the current process; a relative path is
	// resolved from Dir.
	Command string
	// Args are the program's arguments, excluding the program name.
	Args []string
	// Dir is the program's working directory. Empty uses the current
	// process's working directory.
	Dir string
	// Env holds KEY=VALUE entries added to the program's environment. TERM
	// cannot be set because it always names the session's terminal profile.
	Env []string
	// ClearEnv starts from an empty environment instead of inheriting the
	// current process's environment.
	ClearEnv bool
}

// Connector starts local programs on pseudo-terminals.
type Connector struct {
	config Config
}

// NewConnector creates a PTY connector.
func NewConnector(config Config) (*Connector, error) {
	if config.Command == "" {
		return nil, fmt.Errorf("creating PTY connector: command is required")
	}
	for index, entry := range config.Env {
		key, _, found := strings.Cut(entry, "=")
		if !found || key == "" {
			// Name the position rather than the entry, which may hold a secret.
			return nil, fmt.Errorf("creating PTY connector: environment entry %d must have the form KEY=VALUE", index)
		}
		if key == "TERM" {
			return nil, fmt.Errorf("creating PTY connector: TERM is set from the session's terminal profile")
		}
	}
	config.Args = append([]string(nil), config.Args...)
	config.Env = append([]string(nil), config.Env...)
	return &Connector{config: config}, nil
}

// Connect resolves the program so a missing or non-executable command fails
// before any session opens. It starts no process; each session starts its
// own.
func (c *Connector) Connect(ctx context.Context) (tuicast.SessionOpener, error) {
	if !supported {
		return nil, fmt.Errorf("connecting PTY: %w", errUnsupported)
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("connecting PTY: %w", ctx.Err())
	default:
	}
	if c.config.Dir != "" {
		info, err := os.Stat(c.config.Dir)
		if err != nil {
			return nil, fmt.Errorf("checking PTY working directory: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("checking PTY working directory: %s is not a directory", c.config.Dir)
		}
	}
	path := c.config.Command
	if strings.ContainsRune(path, filepath.Separator) && !filepath.IsAbs(path) && c.config.Dir != "" {
		path = filepath.Join(c.config.Dir, path)
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return nil, fmt.Errorf("resolving PTY command: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		if resolved, err = filepath.Abs(resolved); err != nil {
			return nil, fmt.Errorf("resolving PTY command: %w", err)
		}
	}

	environment := c.config.Env
	if !c.config.ClearEnv {
		environment = append(inheritedEnvironment(), environment...)
	}
	return &connection{
		path:        resolved,
		args:        c.config.Args,
		dir:         c.config.Dir,
		environment: environment,
		streams:     make(map[*stream]struct{}),
	}, nil
}

// inheritedEnvironment returns the current environment without the variables
// that describe the terminal, which come from the session instead.
func inheritedEnvironment() []string {
	var environment []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "TERM" || key == "LINES" || key == "COLUMNS" {
			continue
		}
		environment = append(environment, entry)
	}
	return environment
}

type connection struct {
	mu sync.Mutex

	path        string
	args        []string
	dir         string
	environment []string
	streams     map[*stream]struct{}
	closed      bool
}

// OpenSession starts a new instance of the program. A connection may run
// several sessions, each in its own process.
func (c *connection) OpenSession(ctx context.Context, request tuicast.SessionRequest) (tuicast.SessionStream, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("opening PTY session: %w", ctx.Err())
	default:
	}
	if request.TerminalType == "" {
		return nil, fmt.Errorf("opening PTY session: terminal type is required")
	}
	if request.Width <= 0 || request.Height <= 0 {
		return nil, fmt.Errorf("opening PTY session: dimensions must be positive")
	}
	if request.Width > maximumDimension || request.Height > maximumDimension {
		return nil, fmt.Errorf("opening PTY session: dimensions must not exceed %d", maximumDimension)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, fmt.Errorf("opening PTY session: connection is closed")
	}
	cmd := exec.Command(c.path, c.args...)
	cmd.Dir = c.dir
	cmd.Env = append(append([]string(nil), c.environment...), "TERM="+request.TerminalType)
	opened, err := startStream(cmd, request.Width, request.Height, c.removeStream)
	if err != nil {
		return nil, fmt.Errorf("starting %s on a PTY: %w", c.path, err)
	}
	c.streams[opened] = struct{}{}
	return opened, nil
}

// Close ends every session's program.
func (c *connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	streams := make([]*stream, 0, len(c.streams))
	for opened := range c.streams {
		streams = append(streams, opened)
	}
	c.mu.Unlock()

	var closeErrors []error
	for _, opened := range streams {
		if err := opened.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("closing PTY session: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}

func (c *connection) removeStream(closed *stream) {
	c.mu.Lock()
	delete(c.streams, closed)
	c.mu.Unlock()
}

// maximumDimension is the largest size a PTY window size field can hold.
const maximumDimension = 65535
