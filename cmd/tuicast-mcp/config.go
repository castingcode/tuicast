package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/mcpserver"
	tuicastssh "github.com/castingcode/tuicast/ssh"
	"github.com/castingcode/tuicast/telnet"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	defaultConnectTimeout = 30 * time.Second
	maximumConnectTimeout = 10 * time.Minute
	maximumDimension      = 1000
	secretCommandTimeout  = time.Minute
	maximumSecretBytes    = 64 << 10
)

type profileFile struct {
	Profiles map[string]profileConfig `json:"profiles"`
}

type profileConfig struct {
	Description string     `json:"description,omitempty"`
	Protocol    string     `json:"protocol"`
	Address     string     `json:"address"`
	Terminal    string     `json:"terminal"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	SSH         *sshConfig `json:"ssh,omitempty"`
}

type sshConfig struct {
	Username                   string `json:"username"`
	PasswordEnv                string `json:"passwordEnv,omitempty"`
	PrivateKeyFile             string `json:"privateKeyFile,omitempty"`
	PrivateKeyPassphraseEnv    string `json:"privateKeyPassphraseEnv,omitempty"`
	KnownHostsFile             string `json:"knownHostsFile,omitempty"`
	HostKeyFingerprint         string `json:"hostKeyFingerprint,omitempty"`
	InsecureSkipHostKeyCheck   bool   `json:"insecureSkipHostKeyCheck,omitempty"`
	ConnectTimeoutMilliseconds int    `json:"connectTimeoutMilliseconds,omitempty"`
}

func loadProfiles(path, secretCommand string, logger *slog.Logger) ([]mcpserver.Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening TUICast MCP config: %w", err)
	}
	defer file.Close()

	var config profileFile
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding TUICast MCP config: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return nil, err
	}
	if len(config.Profiles) == 0 {
		return nil, fmt.Errorf("validating TUICast MCP config: at least one profile is required")
	}

	baseDirectory := filepath.Dir(path)
	profiles := make([]mcpserver.Profile, 0, len(config.Profiles))
	for name, configured := range config.Profiles {
		profile, err := makeProfile(name, configured, baseDirectory, secretCommand, logger)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decoding TUICast MCP config: multiple JSON values are not allowed")
		}
		return fmt.Errorf("decoding TUICast MCP config: %w", err)
	}
	return nil
}

func makeProfile(name string, configured profileConfig, baseDirectory, secretCommand string, logger *slog.Logger) (mcpserver.Profile, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name {
		return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP config: profile name %q is invalid", name)
	}
	if configured.Address == "" {
		return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP profile %q: address is required", name)
	}
	terminalProfile := tuicast.TerminalProfile(configured.Terminal)
	if terminalProfile != tuicast.ProfileVT220 && terminalProfile != tuicast.ProfileXTerm {
		return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP profile %q: unsupported terminal %q", name, configured.Terminal)
	}
	if configured.Width <= 0 || configured.Height <= 0 || configured.Width > maximumDimension || configured.Height > maximumDimension {
		return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP profile %q: dimensions must be between 1 and %d", name, maximumDimension)
	}

	var connector tuicast.Connector
	var err error
	switch configured.Protocol {
	case "telnet":
		if configured.SSH != nil {
			return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP profile %q: ssh settings are not valid for Telnet", name)
		}
		connector, err = telnet.NewConnector(telnet.Config{Address: configured.Address})
	case "ssh":
		if configured.SSH == nil {
			return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP profile %q: ssh settings are required", name)
		}
		connector, err = makeSSHConnector(name, configured.Address, *configured.SSH, baseDirectory, secretCommand, logger)
	default:
		return mcpserver.Profile{}, fmt.Errorf("validating TUICast MCP profile %q: unsupported protocol %q", name, configured.Protocol)
	}
	if err != nil {
		return mcpserver.Profile{}, err
	}
	return mcpserver.Profile{
		Name: name, Description: configured.Description, Protocol: configured.Protocol,
		Terminal: terminalProfile, Width: configured.Width, Height: configured.Height, Connector: connector,
	}, nil
}

func makeSSHConnector(profileName, address string, configured sshConfig, baseDirectory, secretCommand string, logger *slog.Logger) (tuicast.Connector, error) {
	if configured.Username == "" {
		return nil, fmt.Errorf("validating TUICast MCP profile %q: SSH username is required", profileName)
	}
	var authentication []gossh.AuthMethod
	if configured.PasswordEnv != "" {
		password, err := resolveSecret(profileName, "SSH password", configured.PasswordEnv, secretCommand, logger)
		if err != nil {
			return nil, err
		}
		authentication = append(authentication, gossh.PasswordCallback(password.get))
	}
	if configured.PrivateKeyFile != "" {
		keyPath := resolvePath(baseDirectory, configured.PrivateKeyFile)
		privateKey, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("reading TUICast MCP profile %q SSH private key: %w", profileName, err)
		}
		if configured.PrivateKeyPassphraseEnv == "" {
			signer, err := gossh.ParsePrivateKey(privateKey)
			if err != nil {
				return nil, fmt.Errorf("parsing TUICast MCP profile %q SSH private key: %w", profileName, err)
			}
			authentication = append(authentication, gossh.PublicKeys(signer))
		} else {
			passphrase, err := resolveSecret(profileName, "SSH key passphrase", configured.PrivateKeyPassphraseEnv, secretCommand, logger)
			if err != nil {
				return nil, err
			}
			signers := func() ([]gossh.Signer, error) {
				value, err := passphrase.get()
				if err != nil {
					return nil, err
				}
				signer, err := gossh.ParsePrivateKeyWithPassphrase(privateKey, []byte(value))
				if err != nil {
					return nil, fmt.Errorf("parsing TUICast MCP profile %q SSH private key: %w", profileName, err)
				}
				return []gossh.Signer{signer}, nil
			}
			if passphrase.deferred {
				authentication = append(authentication, gossh.PublicKeysCallback(signers))
			} else {
				parsed, err := signers()
				if err != nil {
					return nil, err
				}
				authentication = append(authentication, gossh.PublicKeys(parsed...))
			}
		}
	} else if configured.PrivateKeyPassphraseEnv != "" {
		return nil, fmt.Errorf("validating TUICast MCP profile %q: privateKeyPassphraseEnv requires privateKeyFile", profileName)
	}
	if len(authentication) == 0 {
		return nil, fmt.Errorf("validating TUICast MCP profile %q: passwordEnv or privateKeyFile is required", profileName)
	}

	hostKeyCallback, err := makeHostKeyCallback(profileName, configured, baseDirectory, logger)
	if err != nil {
		return nil, err
	}
	timeout := defaultConnectTimeout
	if configured.ConnectTimeoutMilliseconds < 0 || configured.ConnectTimeoutMilliseconds > int(maximumConnectTimeout/time.Millisecond) {
		return nil, fmt.Errorf("validating TUICast MCP profile %q: connect timeout must not exceed %s", profileName, maximumConnectTimeout)
	}
	if configured.ConnectTimeoutMilliseconds > 0 {
		timeout = time.Duration(configured.ConnectTimeoutMilliseconds) * time.Millisecond
	}
	connector, err := tuicastssh.NewConnector(tuicastssh.Config{
		Address: address,
		ClientConfig: &gossh.ClientConfig{
			User: configured.Username, Auth: authentication,
			HostKeyCallback: hostKeyCallback, Timeout: timeout,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating TUICast MCP profile %q SSH connector: %w", profileName, err)
	}
	return connector, nil
}

func makeHostKeyCallback(profileName string, configured sshConfig, baseDirectory string, logger *slog.Logger) (gossh.HostKeyCallback, error) {
	methods := 0
	if configured.KnownHostsFile != "" {
		methods++
	}
	if configured.HostKeyFingerprint != "" {
		methods++
	}
	if configured.InsecureSkipHostKeyCheck {
		methods++
	}
	if methods != 1 {
		return nil, fmt.Errorf("validating TUICast MCP profile %q: exactly one SSH host-key verification option is required", profileName)
	}
	switch {
	case configured.KnownHostsFile != "":
		callback, err := knownhosts.New(resolvePath(baseDirectory, configured.KnownHostsFile))
		if err != nil {
			return nil, fmt.Errorf("loading TUICast MCP profile %q known-hosts file: %w", profileName, err)
		}
		return callback, nil
	case configured.HostKeyFingerprint != "":
		expected := configured.HostKeyFingerprint
		return func(_ string, _ net.Addr, key gossh.PublicKey) error {
			actual := gossh.FingerprintSHA256(key)
			if actual != expected {
				return fmt.Errorf("verifying SSH host key: expected %s, received %s", expected, actual)
			}
			return nil
		}, nil
	default:
		logger.Warn("TUICast MCP SSH profile disables host-key verification", "profile", profileName)
		return gossh.InsecureIgnoreHostKey(), nil
	}
}

// secret yields one credential. Values from the environment or a file are
// read at startup. A secret command runs on each connection attempt instead,
// so it can wait for an unlock prompt or return a rotated credential without
// delaying the MCP handshake.
type secret struct {
	get      func() (string, error)
	deferred bool
}

// resolveSecret finds the secret named name in a fixed order: the NAME
// environment variable, then the file named by NAME_FILE, then the secret
// command, which receives name as its only argument. Empty variables count as
// unset. Only the source is logged, never the value.
func resolveSecret(profileName, purpose, name, secretCommand string, logger *slog.Logger) (secret, error) {
	fileVariable := name + "_FILE"
	value, path := os.Getenv(name), os.Getenv(fileVariable)
	var resolved secret
	var source string
	switch {
	case value != "" && path != "":
		return secret{}, fmt.Errorf("loading TUICast MCP profile %q %s: environment variables %q and %q are both set; set only one", profileName, purpose, name, fileVariable)
	case value != "":
		resolved, source = staticSecret(value), "environment"
	case path != "":
		contents, err := readSecretFile(path)
		if err != nil {
			return secret{}, fmt.Errorf("loading TUICast MCP profile %q %s from %s: %w", profileName, purpose, fileVariable, err)
		}
		resolved, source = staticSecret(contents), "file"
	case secretCommand != "":
		source = "command"
		resolved = secret{deferred: true, get: func() (string, error) {
			value, err := runSecretCommand(secretCommand, name)
			if err != nil {
				logger.Warn("TUICast MCP secret command failed", "profile", profileName, "secret", name, "error", err)
				return "", fmt.Errorf("loading TUICast MCP profile %q %s: %w", profileName, purpose, err)
			}
			return value, nil
		}}
	default:
		return secret{}, fmt.Errorf("loading TUICast MCP profile %q %s: environment variable %q is not set; set it, set %q to the path of a file containing it, or start tuicast-mcp with -secret-command", profileName, purpose, name, fileVariable)
	}
	logger.Info("TUICast MCP profile secret configured", "profile", profileName, "secret", name, "source", source)
	return resolved, nil
}

func staticSecret(value string) secret {
	return secret{get: func() (string, error) { return value, nil }}
}

func readSecretFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening secret file: %w", err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maximumSecretBytes+1))
	if err != nil {
		return "", fmt.Errorf("reading secret file: %w", err)
	}
	return trimSecret(contents)
}

func runSecretCommand(command, name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), secretCommandTimeout)
	defer cancel()
	// Stdin stays nil, so the command reads the null device and cannot consume
	// MCP protocol messages from this process's standard input. Output
	// captures stdout rather than letting it corrupt the protocol stream.
	cmd := exec.CommandContext(ctx, command, name)
	cmd.WaitDelay = time.Second
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("running secret command for %q: timed out after %s", name, secretCommandTimeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if message := strings.TrimSpace(string(exitErr.Stderr)); message != "" {
				if len(message) > 512 {
					message = message[:512] + "..."
				}
				return "", fmt.Errorf("running secret command for %q: %w: %s", name, err, message)
			}
		}
		return "", fmt.Errorf("running secret command for %q: %w", name, err)
	}
	value, err := trimSecret(output)
	if err != nil {
		return "", fmt.Errorf("reading secret command output for %q: %w", name, err)
	}
	return value, nil
}

// trimSecret removes one trailing line ending. Other whitespace is kept
// because it can be part of a password.
func trimSecret(raw []byte) (string, error) {
	if len(raw) > maximumSecretBytes {
		return "", fmt.Errorf("secret exceeds %d bytes", maximumSecretBytes)
	}
	value := string(raw)
	if strings.HasSuffix(value, "\r\n") {
		value = value[:len(value)-2]
	} else {
		value = strings.TrimSuffix(value, "\n")
	}
	if value == "" {
		return "", fmt.Errorf("secret is empty")
	}
	return value, nil
}

func resolvePath(baseDirectory, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDirectory, path)
}
