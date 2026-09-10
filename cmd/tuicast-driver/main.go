package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/driver"
	"github.com/castingcode/tuicast/driverui"
	tuicastssh "github.com/castingcode/tuicast/ssh"
	"github.com/castingcode/tuicast/telnet"
	"github.com/castingcode/tuicast/vt220"
	"github.com/castingcode/tuicast/workbench"
	"github.com/castingcode/tuicast/xterm"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(os.Args[1:], os.Stdin, os.Stdout, logger); err != nil {
		logger.Error("TUICast driver stopped", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string, input io.Reader, output io.Writer, logger *slog.Logger) error {
	flags := flag.NewFlagSet("tuicast-driver", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	uiAddress := flags.String("ui-address", "", "TUICast Inspector listen address (for example 127.0.0.1:0)")
	workbenchEnabled := flags.Bool("workbench", false, "enable TUICast Workbench on the loopback Inspector listener")
	if err := flags.Parse(arguments); err != nil {
		return fmt.Errorf("parsing TUICast driver flags: %w", err)
	}
	if *workbenchEnabled && *uiAddress == "" {
		return fmt.Errorf("configuring TUICast Workbench: -ui-address is required")
	}
	if *workbenchEnabled && !isLoopbackAddress(*uiAddress) {
		return fmt.Errorf("configuring TUICast Workbench: -ui-address must be a loopback address")
	}

	server, err := driver.New(logger, connector, terminal)
	if err != nil {
		return fmt.Errorf("initializing TUICast driver: %w", err)
	}

	var inspector *driverui.Server
	var serveResult chan error
	if *uiAddress != "" {
		listener, err := net.Listen("tcp", *uiAddress)
		if err != nil {
			return errors.Join(fmt.Errorf("listening for TUICast Inspector: %w", err), server.Close())
		}
		var options []driverui.Option
		if *workbenchEnabled {
			manager, managerErr := workbench.New(func(id uint64) (workbench.SessionControl, error) {
				return server.AcquireSessionControl(id)
			})
			if managerErr != nil {
				return errors.Join(managerErr, listener.Close(), server.Close())
			}
			options = append(options, driverui.WithWorkbench(manager))
		}
		inspector, err = driverui.New(server, options...)
		if err != nil {
			return errors.Join(err, listener.Close(), server.Close())
		}
		if !isLoopbackAddress(*uiAddress) {
			logger.Warn("TUICast Inspector is listening without authentication or TLS", "address", listener.Addr())
		}
		logger.Info("TUICast Inspector listening", "url", "http://"+listener.Addr().String())
		if *workbenchEnabled {
			logger.Info("TUICast Workbench enabled", "url", "http://"+listener.Addr().String())
		}
		serveResult = make(chan error, 1)
		go func() { serveResult <- inspector.Serve(listener) }()
	}

	runErr := server.Run(input, output)
	if inspector == nil {
		return runErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := inspector.Shutdown(ctx)
	serveErr := <-serveResult
	return errors.Join(runErr, shutdownErr, serveErr)
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func connector(options driver.ConnectionOptions) (tuicast.Connector, error) {
	switch options.Protocol {
	case "telnet":
		opened, err := telnet.NewConnector(telnet.Config{Address: options.Address})
		if err != nil {
			return nil, fmt.Errorf("creating Telnet connector: %w", err)
		}
		return opened, nil
	case "ssh":
		return sshConnector(options)
	default:
		return nil, fmt.Errorf("creating connector: unsupported protocol %q", options.Protocol)
	}
}

func sshConnector(options driver.ConnectionOptions) (tuicast.Connector, error) {
	if options.Username == "" {
		return nil, fmt.Errorf("creating SSH connector: username is required")
	}
	var authentication []gossh.AuthMethod
	if options.Password != "" {
		authentication = append(authentication, gossh.Password(options.Password))
	}
	if options.PrivateKey != "" {
		signer, err := parsePrivateKey(options.PrivateKey, options.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		authentication = append(authentication, gossh.PublicKeys(signer))
	}
	if len(authentication) == 0 {
		return nil, fmt.Errorf("creating SSH connector: password or privateKey is required")
	}

	hostKeyCallback, err := makeHostKeyCallback(options)
	if err != nil {
		return nil, err
	}
	timeout := 30 * time.Second
	if options.ConnectTimeoutMilliseconds > 0 {
		timeout = time.Duration(options.ConnectTimeoutMilliseconds) * time.Millisecond
	}
	opened, err := tuicastssh.NewConnector(tuicastssh.Config{
		Address: options.Address,
		ClientConfig: &gossh.ClientConfig{
			User:            options.Username,
			Auth:            authentication,
			HostKeyCallback: hostKeyCallback,
			Timeout:         timeout,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating SSH connector: %w", err)
	}
	return opened, nil
}

func parsePrivateKey(privateKey, passphrase string) (gossh.Signer, error) {
	var signer gossh.Signer
	var err error
	if passphrase == "" {
		signer, err = gossh.ParsePrivateKey([]byte(privateKey))
	} else {
		signer, err = gossh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
	}
	if err != nil {
		return nil, fmt.Errorf("parsing SSH private key: %w", err)
	}
	return signer, nil
}

func makeHostKeyCallback(options driver.ConnectionOptions) (gossh.HostKeyCallback, error) {
	configured := 0
	if options.KnownHostsFile != "" {
		configured++
	}
	if options.HostKeyFingerprint != "" {
		configured++
	}
	if options.InsecureSkipHostKeyCheck {
		configured++
	}
	if configured != 1 {
		return nil, fmt.Errorf("creating SSH connector: exactly one host-key verification option is required")
	}

	switch {
	case options.KnownHostsFile != "":
		callback, err := knownhosts.New(options.KnownHostsFile)
		if err != nil {
			return nil, fmt.Errorf("loading SSH known-hosts file: %w", err)
		}
		return callback, nil
	case options.HostKeyFingerprint != "":
		expected := options.HostKeyFingerprint
		return func(_ string, _ net.Addr, key gossh.PublicKey) error {
			actual := gossh.FingerprintSHA256(key)
			if actual != expected {
				return fmt.Errorf("verifying SSH host key: expected %s, received %s", expected, actual)
			}
			return nil
		}, nil
	default:
		return gossh.InsecureIgnoreHostKey(), nil
	}
}

func terminal(profile string, width, height int) (tuicast.Terminal, error) {
	switch tuicast.TerminalProfile(profile) {
	case tuicast.ProfileVT220:
		created, err := vt220.New(width, height)
		if err != nil {
			return nil, fmt.Errorf("creating VT220 terminal: %w", err)
		}
		return created, nil
	case tuicast.ProfileXTerm:
		created, err := xterm.New(width, height)
		if err != nil {
			return nil, fmt.Errorf("creating xterm terminal: %w", err)
		}
		return created, nil
	default:
		return nil, fmt.Errorf("creating terminal: unsupported profile %q", profile)
	}
}
