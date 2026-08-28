// Package ssh implements SSH transport using golang.org/x/crypto/ssh.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/castingcode/tuicast"
	xssh "golang.org/x/crypto/ssh"
)

var _ tuicast.Connector = (*Connector)(nil)
var _ tuicast.SessionOpener = (*connection)(nil)
var _ tuicast.SessionStream = (*stream)(nil)

// DialContextFunc establishes the TCP connection used by SSH.
type DialContextFunc func(context.Context, string, string) (net.Conn, error)

// Config configures an SSH endpoint.
type Config struct {
	Address       string
	ClientConfig  *xssh.ClientConfig
	TerminalModes xssh.TerminalModes
	DialContext   DialContextFunc
}

// Connector establishes authenticated SSH connections.
type Connector struct {
	address string
	config  *xssh.ClientConfig
	modes   xssh.TerminalModes
	dial    DialContextFunc
}

// NewConnector creates an SSH connector. Host-key verification must be
// configured explicitly through ClientConfig.HostKeyCallback.
func NewConnector(config Config) (*Connector, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("creating SSH connector: address is required")
	}
	if config.ClientConfig == nil {
		return nil, fmt.Errorf("creating SSH connector: client config is required")
	}
	if config.ClientConfig.HostKeyCallback == nil {
		return nil, fmt.Errorf("creating SSH connector: host key callback is required")
	}

	clientConfig := *config.ClientConfig
	clientConfig.Auth = append([]xssh.AuthMethod(nil), config.ClientConfig.Auth...)
	clientConfig.HostKeyAlgorithms = append([]string(nil), config.ClientConfig.HostKeyAlgorithms...)
	clientConfig.KeyExchanges = append([]string(nil), config.ClientConfig.KeyExchanges...)
	clientConfig.Ciphers = append([]string(nil), config.ClientConfig.Ciphers...)
	clientConfig.MACs = append([]string(nil), config.ClientConfig.MACs...)
	modes := make(xssh.TerminalModes, len(config.TerminalModes))
	for mode, value := range config.TerminalModes {
		modes[mode] = value
	}
	dial := config.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: clientConfig.Timeout}).DialContext
	}

	return &Connector{
		address: config.Address,
		config:  &clientConfig,
		modes:   modes,
		dial:    dial,
	}, nil
}

// Connect establishes and authenticates one multiplexed SSH connection.
func (c *Connector) Connect(ctx context.Context) (tuicast.SessionOpener, error) {
	raw, err := c.dial(ctx, "tcp", c.address)
	if err != nil {
		return nil, fmt.Errorf("dialing SSH server %s: %w", c.address, err)
	}

	clientConnection, channels, requests, err := newClientConn(ctx, raw, c.address, c.config)
	if err != nil {
		if closeErr := raw.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) && !errors.Is(closeErr, io.ErrClosedPipe) {
			return nil, errors.Join(
				fmt.Errorf("handshaking with SSH server %s: %w", c.address, err),
				fmt.Errorf("closing failed SSH connection: %w", closeErr),
			)
		}
		return nil, fmt.Errorf("handshaking with SSH server %s: %w", c.address, err)
	}

	return &connection{
		client: xssh.NewClient(clientConnection, channels, requests),
		modes:  c.modes,
	}, nil
}

type clientConnectionResult struct {
	connection xssh.Conn
	channels   <-chan xssh.NewChannel
	requests   <-chan *xssh.Request
	err        error
}

func newClientConn(ctx context.Context, raw net.Conn, address string, config *xssh.ClientConfig) (xssh.Conn, <-chan xssh.NewChannel, <-chan *xssh.Request, error) {
	result := make(chan clientConnectionResult, 1)
	go func() {
		connection, channels, requests, err := xssh.NewClientConn(raw, address, config)
		result <- clientConnectionResult{
			connection: connection,
			channels:   channels,
			requests:   requests,
			err:        err,
		}
	}()

	select {
	case completed := <-result:
		return completed.connection, completed.channels, completed.requests, completed.err
	case <-ctx.Done():
		closeErr := raw.Close()
		<-result
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			return nil, nil, nil, errors.Join(
				fmt.Errorf("canceling SSH handshake: %w", ctx.Err()),
				fmt.Errorf("closing canceled SSH connection: %w", closeErr),
			)
		}
		return nil, nil, nil, fmt.Errorf("canceling SSH handshake: %w", ctx.Err())
	}
}

type connection struct {
	client *xssh.Client
	modes  xssh.TerminalModes
}

func (c *connection) OpenSession(ctx context.Context, request tuicast.SessionRequest) (tuicast.SessionStream, error) {
	if request.Width <= 0 || request.Height <= 0 {
		return nil, fmt.Errorf("opening SSH session: dimensions must be positive")
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("opening SSH session: %w", ctx.Err())
	default:
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("opening SSH channel: %w", err)
	}
	input, err := session.StdinPipe()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("opening SSH input pipe: %w", err),
			cleanupSession(session),
		)
	}
	output, outputWriter := io.Pipe()
	session.Stdout = outputWriter
	session.Stderr = outputWriter

	if err := session.RequestPty(request.TerminalType, request.Height, request.Width, c.modes); err != nil {
		return nil, errors.Join(
			fmt.Errorf("requesting SSH pseudo-terminal: %w", err),
			cleanupSession(session, input, outputWriter, output),
		)
	}
	if err := session.Shell(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("starting SSH shell: %w", err),
			cleanupSession(session, input, outputWriter, output),
		)
	}

	opened := &stream{
		session:      session,
		input:        input,
		output:       output,
		outputWriter: outputWriter,
	}
	go opened.wait()
	return opened, nil
}

func (c *connection) Close() error {
	if err := c.client.Close(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("closing SSH client: %w", err)
	}
	return nil
}

type stream struct {
	session      *xssh.Session
	input        io.WriteCloser
	output       *io.PipeReader
	outputWriter *io.PipeWriter

	closeOnce sync.Once
	closeErr  error
	waitMu    sync.Mutex
	waitErr   error
}

func (s *stream) Read(data []byte) (int, error) {
	return s.output.Read(data)
}

func (s *stream) Write(data []byte) (int, error) {
	return s.input.Write(data)
}

func (s *stream) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("resizing SSH session: dimensions must be positive")
	}
	if err := s.session.WindowChange(height, width); err != nil {
		return fmt.Errorf("sending SSH window change: %w", err)
	}
	return nil
}

func (s *stream) Close() error {
	s.closeOnce.Do(func() {
		var closeErrors []error
		if err := s.input.Close(); err != nil && !errors.Is(err, io.EOF) {
			closeErrors = append(closeErrors, fmt.Errorf("closing SSH input: %w", err))
		}
		if err := s.session.Close(); err != nil && !errors.Is(err, io.EOF) {
			closeErrors = append(closeErrors, fmt.Errorf("closing SSH session: %w", err))
		}
		if err := s.outputWriter.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			closeErrors = append(closeErrors, fmt.Errorf("closing SSH output writer: %w", err))
		}
		if err := s.output.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			closeErrors = append(closeErrors, fmt.Errorf("closing SSH output: %w", err))
		}
		s.closeErr = errors.Join(closeErrors...)
	})
	s.waitMu.Lock()
	waitErr := s.waitErr
	s.waitMu.Unlock()
	return errors.Join(s.closeErr, waitErr)
}

func (s *stream) wait() {
	err := s.session.Wait()
	var closeErr error
	if err != nil {
		closeErr = s.outputWriter.CloseWithError(fmt.Errorf("waiting for SSH session: %w", err))
	} else {
		closeErr = s.outputWriter.Close()
	}
	if closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
		s.waitMu.Lock()
		s.waitErr = fmt.Errorf("closing completed SSH output: %w", closeErr)
		s.waitMu.Unlock()
	}
}

func cleanupSession(session *xssh.Session, resources ...io.Closer) error {
	var cleanupErrors []error
	for _, resource := range resources {
		if err := resource.Close(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("closing failed SSH session resource: %w", err))
		}
	}
	if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("closing failed SSH session: %w", err))
	}
	return errors.Join(cleanupErrors...)
}
