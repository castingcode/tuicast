package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/driver"
	tuicastssh "github.com/castingcode/tuicast/ssh"
	"github.com/castingcode/tuicast/telnet"
	"github.com/castingcode/tuicast/vt220"
	"github.com/castingcode/tuicast/xterm"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server, err := driver.New(logger, connector, terminal)
	if err != nil {
		logger.Error("TUICast driver initialization failed", "error", err)
		os.Exit(1)
	}
	if err := server.Run(os.Stdin, os.Stdout); err != nil {
		logger.Error("TUICast driver stopped", "error", err)
		os.Exit(1)
	}
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
