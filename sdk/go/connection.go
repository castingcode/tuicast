package tuicast

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConnectionConfig describes one supported transport connection.
type ConnectionConfig interface {
	driverParams(time.Duration) (map[string]any, time.Duration, error)
}

// Telnet configures a Telnet connection.
type Telnet struct {
	Address string
	Timeout time.Duration
}

func (config Telnet) driverParams(defaultTimeout time.Duration) (map[string]any, time.Duration, error) {
	if config.Address == "" {
		return nil, 0, fmt.Errorf("configuring Telnet connection: address is required")
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	milliseconds, err := positiveMilliseconds(timeout)
	if err != nil {
		return nil, 0, fmt.Errorf("configuring Telnet connection timeout: %w", err)
	}
	return map[string]any{
		"protocol":                   "telnet",
		"address":                    config.Address,
		"connectTimeoutMilliseconds": milliseconds,
	}, timeout, nil
}

// SSH configures an SSH connection. Password or PrivateKey is required, along
// with exactly one host-key verification option.
type SSH struct {
	Address                  string
	Username                 string
	Password                 string
	PrivateKey               string
	PrivateKeyPassphrase     string
	KnownHostsFile           string
	HostKeyFingerprint       string
	InsecureSkipHostKeyCheck bool
	Timeout                  time.Duration
}

func (config SSH) driverParams(defaultTimeout time.Duration) (map[string]any, time.Duration, error) {
	if config.Address == "" {
		return nil, 0, fmt.Errorf("configuring SSH connection: address is required")
	}
	if config.Username == "" {
		return nil, 0, fmt.Errorf("configuring SSH connection: username is required")
	}
	if config.Password == "" && config.PrivateKey == "" {
		return nil, 0, fmt.Errorf("configuring SSH connection: password or private key is required")
	}
	verificationOptions := 0
	if config.KnownHostsFile != "" {
		verificationOptions++
	}
	if config.HostKeyFingerprint != "" {
		verificationOptions++
	}
	if config.InsecureSkipHostKeyCheck {
		verificationOptions++
	}
	if verificationOptions != 1 {
		return nil, 0, fmt.Errorf("configuring SSH connection: exactly one host-key verification option is required")
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	milliseconds, err := positiveMilliseconds(timeout)
	if err != nil {
		return nil, 0, fmt.Errorf("configuring SSH connection timeout: %w", err)
	}
	return map[string]any{
		"protocol":                   "ssh",
		"address":                    config.Address,
		"username":                   config.Username,
		"password":                   config.Password,
		"privateKey":                 config.PrivateKey,
		"privateKeyPassphrase":       config.PrivateKeyPassphrase,
		"knownHostsFile":             config.KnownHostsFile,
		"hostKeyFingerprint":         config.HostKeyFingerprint,
		"insecureSkipHostKeyCheck":   config.InsecureSkipHostKeyCheck,
		"connectTimeoutMilliseconds": milliseconds,
	}, timeout, nil
}

// Connection is a driver-managed transport connection.
type Connection struct {
	client *Driver
	id     uint64

	closeMu sync.Mutex
	closed  bool
}

// ID returns the driver's connection identifier.
func (c *Connection) ID() uint64 { return c.id }

// Connect opens a typed SSH or Telnet connection.
func (c *Driver) Connect(ctx context.Context, config ConnectionConfig) (*Connection, error) {
	if config == nil {
		return nil, fmt.Errorf("opening connection: configuration is required")
	}
	params, timeout, err := config.driverParams(c.defaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("opening connection: %w", err)
	}
	operationContext, cancel := c.driverOperationContext(ctx, timeout)
	defer cancel()
	var result struct {
		ConnectionID uint64 `json:"connectionId"`
	}
	if err := c.call(operationContext, "connection.open", params, &result); err != nil {
		return nil, fmt.Errorf("opening connection: %w", err)
	}
	return &Connection{client: c, id: result.ConnectionID}, nil
}

// Close closes the connection and every session opened from it. It is
// idempotent.
func (c *Connection) Close(ctx context.Context) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return nil
	}
	operationContext, cancel := c.client.operationContext(ctx, c.client.defaultTimeout)
	defer cancel()
	if err := c.client.call(operationContext, "connection.close", map[string]any{"connectionId": c.id}, nil); err != nil {
		return fmt.Errorf("closing connection %d: %w", c.id, err)
	}
	c.closed = true
	return nil
}

func positiveMilliseconds(duration time.Duration) (int64, error) {
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	milliseconds := duration / time.Millisecond
	if duration%time.Millisecond != 0 {
		milliseconds++
	}
	return int64(milliseconds), nil
}
