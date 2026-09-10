package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
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

func loadProfiles(path string, logger *slog.Logger) ([]mcpserver.Profile, error) {
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
		profile, err := makeProfile(name, configured, baseDirectory, logger)
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

func makeProfile(name string, configured profileConfig, baseDirectory string, logger *slog.Logger) (mcpserver.Profile, error) {
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
		connector, err = makeSSHConnector(name, configured.Address, *configured.SSH, baseDirectory, logger)
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

func makeSSHConnector(profileName, address string, configured sshConfig, baseDirectory string, logger *slog.Logger) (tuicast.Connector, error) {
	if configured.Username == "" {
		return nil, fmt.Errorf("validating TUICast MCP profile %q: SSH username is required", profileName)
	}
	var authentication []gossh.AuthMethod
	if configured.PasswordEnv != "" {
		password, err := requiredEnvironment(configured.PasswordEnv)
		if err != nil {
			return nil, fmt.Errorf("loading TUICast MCP profile %q SSH password: %w", profileName, err)
		}
		authentication = append(authentication, gossh.Password(password))
	}
	if configured.PrivateKeyFile != "" {
		keyPath := resolvePath(baseDirectory, configured.PrivateKeyFile)
		privateKey, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("reading TUICast MCP profile %q SSH private key: %w", profileName, err)
		}
		var signer gossh.Signer
		if configured.PrivateKeyPassphraseEnv == "" {
			signer, err = gossh.ParsePrivateKey(privateKey)
		} else {
			passphrase, envErr := requiredEnvironment(configured.PrivateKeyPassphraseEnv)
			if envErr != nil {
				return nil, fmt.Errorf("loading TUICast MCP profile %q SSH key passphrase: %w", profileName, envErr)
			}
			signer, err = gossh.ParsePrivateKeyWithPassphrase(privateKey, []byte(passphrase))
		}
		if err != nil {
			return nil, fmt.Errorf("parsing TUICast MCP profile %q SSH private key: %w", profileName, err)
		}
		authentication = append(authentication, gossh.PublicKeys(signer))
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

func requiredEnvironment(name string) (string, error) {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", fmt.Errorf("reading environment variable %q: value is required", name)
	}
	return value, nil
}

func resolvePath(baseDirectory, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDirectory, path)
}
