// Package ssh serves the reference TUI over SSH.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/castingcode/tuicast/reference"
	gossh "golang.org/x/crypto/ssh"
)

// Config configures authentication and host identity for a reference server.
type Config struct {
	Username string
	Password string
	Signer   gossh.Signer
	Logger   *slog.Logger
}

// Serve accepts SSH connections until the context is canceled or the listener
// fails. Each SSH session gets an isolated reference application.
func Serve(ctx context.Context, listener net.Listener, config Config) error {
	if ctx == nil {
		return fmt.Errorf("serving reference TUI over SSH: context is required")
	}
	if listener == nil {
		return fmt.Errorf("serving reference TUI over SSH: listener is required")
	}
	if config.Username == "" || config.Password == "" {
		return fmt.Errorf("serving reference TUI over SSH: username and password are required")
	}
	if config.Signer == nil {
		return fmt.Errorf("serving reference TUI over SSH: host signer is required")
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	serverConfig := &gossh.ServerConfig{
		PasswordCallback: func(metadata gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			if metadata.User() != config.Username || string(password) != config.Password {
				return nil, fmt.Errorf("authenticating SSH user: invalid credentials")
			}
			return nil, nil
		},
	}
	serverConfig.AddHostKey(config.Signer)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accepting SSH connection: %w", err)
		}
		go serveConnection(connection, serverConfig, logger)
	}
}

func serveConnection(raw net.Conn, config *gossh.ServerConfig, logger *slog.Logger) {
	connection, channels, requests, err := gossh.NewServerConn(raw, config)
	if err != nil {
		logger.Warn("SSH handshake failed", "remote", raw.RemoteAddr(), "error", err)
		_ = raw.Close()
		return
	}
	defer connection.Close()
	go gossh.DiscardRequests(requests)
	for requested := range channels {
		if requested.ChannelType() != "session" {
			_ = requested.Reject(gossh.UnknownChannelType, "only session channels are supported")
			continue
		}
		channel, requests, err := requested.Accept()
		if err != nil {
			logger.Warn("SSH session channel failed", "remote", raw.RemoteAddr(), "error", err)
			continue
		}
		go serveSession(channel, requests, logger)
	}
}

func serveSession(channel gossh.Channel, requests <-chan *gossh.Request, logger *slog.Logger) {
	defer channel.Close()
	width, height := 80, 24
	resizes := make(chan reference.Size, 1)
	defer close(resizes)
	applicationDone := make(chan error, 1)
	running := false

	for {
		var request *gossh.Request
		select {
		case err := <-applicationDone:
			if err != nil && !errors.Is(err, io.EOF) {
				logger.Warn("reference SSH session stopped", "error", err)
			}
			return
		case received, ok := <-requests:
			if !ok {
				return
			}
			request = received
		}
		switch request.Type {
		case "pty-req":
			var dimensions struct {
				Terminal    string
				Width       uint32
				Height      uint32
				PixelWidth  uint32
				PixelHeight uint32
				Modes       string
			}
			if err := gossh.Unmarshal(request.Payload, &dimensions); err != nil || dimensions.Width == 0 || dimensions.Height == 0 {
				_ = request.Reply(false, nil)
				continue
			}
			width, height = int(dimensions.Width), int(dimensions.Height)
			_ = request.Reply(true, nil)
		case "shell":
			if running {
				_ = request.Reply(false, nil)
				continue
			}
			running = true
			_ = request.Reply(true, nil)
			go func(width, height int) {
				application, err := reference.New(width, height)
				if err == nil {
					err = application.RunWithResizes(channel, channel, resizes)
				}
				applicationDone <- err
			}(width, height)
		case "window-change":
			var dimensions struct {
				Width       uint32
				Height      uint32
				PixelWidth  uint32
				PixelHeight uint32
			}
			if err := gossh.Unmarshal(request.Payload, &dimensions); err == nil && dimensions.Width > 0 && dimensions.Height > 0 {
				size := reference.Size{Width: int(dimensions.Width), Height: int(dimensions.Height)}
				select {
				case resizes <- size:
				default:
					<-resizes
					resizes <- size
				}
			}
		default:
			_ = request.Reply(false, nil)
		}
	}
}
