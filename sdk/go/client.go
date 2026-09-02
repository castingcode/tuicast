// Package tuicast provides an idiomatic Go client for the TUICast driver.
package tuicast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultDriverPath = "tuicast-driver"
	defaultTimeout    = 30 * time.Second
	protocolVersion   = "1"
)

// LaunchOption configures a driver process.
type LaunchOption func(*launchConfig) error

type launchConfig struct {
	path        string
	args        []string
	stderr      io.Writer
	environment []string
	timeout     time.Duration
}

// WithDriverPath selects the tuicast-driver executable. The default resolves
// tuicast-driver through PATH.
func WithDriverPath(path string) LaunchOption {
	return func(config *launchConfig) error {
		if path == "" {
			return fmt.Errorf("configuring driver path: path is required")
		}
		config.path = path
		return nil
	}
}

// WithDriverArgs appends command-line arguments when launching the driver.
func WithDriverArgs(arguments ...string) LaunchOption {
	return func(config *launchConfig) error {
		config.args = append(config.args, arguments...)
		return nil
	}
}

// WithDriverStderr selects where driver diagnostics are written. The default
// is the parent process's standard error.
func WithDriverStderr(stderr io.Writer) LaunchOption {
	return func(config *launchConfig) error {
		if stderr == nil {
			return fmt.Errorf("configuring driver stderr: writer is required")
		}
		config.stderr = stderr
		return nil
	}
}

// WithDriverEnvironment adds environment entries in KEY=value form. Entries
// override inherited values with the same key.
func WithDriverEnvironment(environment ...string) LaunchOption {
	return func(config *launchConfig) error {
		config.environment = append(config.environment, environment...)
		return nil
	}
}

// WithDefaultTimeout sets the maximum duration for operations whose context
// has no earlier deadline.
func WithDefaultTimeout(timeout time.Duration) LaunchOption {
	return func(config *launchConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("configuring default timeout: duration must be positive")
		}
		config.timeout = timeout
		return nil
	}
}

// Driver owns one driver process and its connections.
type Driver struct {
	command        *exec.Cmd
	input          io.WriteCloser
	encoder        *json.Encoder
	defaultTimeout time.Duration
	processDone    <-chan error

	writeMu  sync.Mutex
	mu       sync.Mutex
	nextID   uint64
	pending  map[uint64]chan rpcReply
	failure  error
	done     chan struct{}
	doneOnce sync.Once

	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
}

type rpcReply struct {
	response rpcResponse
	err      error
}

// RPCError is an error returned by the JSON-RPC driver.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("driver error %d: %s", e.Code, e.Message)
}

// Launch starts a driver, verifies its protocol version, and returns a client.
func Launch(ctx context.Context, options ...LaunchOption) (*Driver, error) {
	if ctx == nil {
		return nil, fmt.Errorf("launching TUICast driver: context is required")
	}
	config := launchConfig{
		path:    defaultDriverPath,
		stderr:  os.Stderr,
		timeout: defaultTimeout,
	}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("launching TUICast driver: launch option is required")
		}
		if err := option(&config); err != nil {
			return nil, fmt.Errorf("launching TUICast driver: %w", err)
		}
	}

	command := exec.Command(config.path, config.args...)
	command.Stderr = config.stderr
	if len(config.environment) > 0 {
		command.Env = append(os.Environ(), config.environment...)
	}
	input, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("opening driver stdin: %w", err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opening driver stdout: %w", err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("starting TUICast driver: %w", err)
	}
	processDone := make(chan error, 1)
	go func() {
		processDone <- command.Wait()
	}()
	client := &Driver{
		command:        command,
		input:          input,
		encoder:        json.NewEncoder(input),
		defaultTimeout: config.timeout,
		processDone:    processDone,
		pending:        make(map[uint64]chan rpcReply),
		done:           make(chan struct{}),
		closeDone:      make(chan struct{}),
	}
	go client.read(json.NewDecoder(output))

	operationContext, cancel := client.operationContext(ctx, config.timeout)
	defer cancel()
	var ping struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := client.call(operationContext, "driver.ping", nil, &ping); err != nil {
		_ = client.stopProcess()
		return nil, fmt.Errorf("checking TUICast driver protocol: %w", err)
	}
	if ping.ProtocolVersion != protocolVersion {
		_ = client.stopProcess()
		return nil, fmt.Errorf("checking TUICast driver protocol: client requires version %s, driver reported %s", protocolVersion, ping.ProtocolVersion)
	}
	return client, nil
}

func (c *Driver) read(decoder *json.Decoder) {
	for {
		var response rpcResponse
		if err := decoder.Decode(&response); err != nil {
			c.fail(fmt.Errorf("reading driver response: %w", err))
			return
		}
		if response.JSONRPC != "2.0" {
			c.fail(fmt.Errorf("reading driver response: unsupported JSON-RPC version %q", response.JSONRPC))
			return
		}
		if response.ID == 0 {
			if response.Method != "" {
				continue
			}
			c.fail(fmt.Errorf("reading driver response: response identifier is required"))
			return
		}
		c.mu.Lock()
		result := c.pending[response.ID]
		delete(c.pending, response.ID)
		c.mu.Unlock()
		if result != nil {
			result <- rpcReply{response: response}
		}
	}
}

func (c *Driver) call(ctx context.Context, method string, params, destination any) error {
	if ctx == nil {
		return fmt.Errorf("calling %s: context is required", method)
	}
	result := make(chan rpcReply, 1)
	c.mu.Lock()
	if c.failure != nil {
		err := c.failure
		c.mu.Unlock()
		return err
	}
	c.nextID++
	id := c.nextID
	c.pending[id] = result
	c.mu.Unlock()

	c.writeMu.Lock()
	err := c.encoder.Encode(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	c.writeMu.Unlock()
	if err != nil {
		c.removePending(id)
		c.fail(fmt.Errorf("writing driver request: %w", err))
		return fmt.Errorf("calling %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		c.removePending(id)
		return fmt.Errorf("calling %s: %w", method, ctx.Err())
	case reply := <-result:
		if reply.err != nil {
			return fmt.Errorf("calling %s: %w", method, reply.err)
		}
		if reply.response.Error != nil {
			return makeClientError(reply.response.Error)
		}
		if destination == nil || len(reply.response.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(reply.response.Result, destination); err != nil {
			return fmt.Errorf("decoding %s result: %w", method, err)
		}
		return nil
	}
}

func (c *Driver) fail(err error) {
	c.mu.Lock()
	if c.failure == nil {
		c.failure = err
	}
	pending := c.pending
	c.pending = make(map[uint64]chan rpcReply)
	c.mu.Unlock()
	for _, result := range pending {
		result <- rpcReply{err: err}
	}
	c.doneOnce.Do(func() { close(c.done) })
}

func (c *Driver) removePending(id uint64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Driver) operationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

func (c *Driver) driverOperationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	const responseGrace = time.Second
	if timeout <= time.Duration(1<<63-1)-responseGrace {
		timeout += responseGrace
	}
	return context.WithTimeout(ctx, timeout)
}

// Close shuts down the driver and waits for its process to exit. It is
// idempotent.
func (c *Driver) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.close()
		close(c.closeDone)
	})
	<-c.closeDone
	return c.closeErr
}

func (c *Driver) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.defaultTimeout)
	defer cancel()
	var shutdownErr error
	select {
	case <-c.done:
	default:
		shutdownErr = c.call(ctx, "driver.shutdown", nil, nil)
	}
	if err := c.input.Close(); err != nil && shutdownErr == nil {
		shutdownErr = fmt.Errorf("closing driver stdin: %w", err)
	}
	select {
	case processErr := <-c.processDone:
		if processErr != nil {
			return errors.Join(shutdownErr, fmt.Errorf("waiting for driver process: %w", processErr))
		}
		return shutdownErr
	case <-ctx.Done():
		killErr := c.command.Process.Kill()
		processErr := <-c.processDone
		return errors.Join(shutdownErr, fmt.Errorf("waiting for driver process: %w", ctx.Err()), killErr, processErr)
	}
}

func (c *Driver) stopProcess() error {
	_ = c.input.Close()
	if c.command.Process != nil {
		_ = c.command.Process.Kill()
	}
	return <-c.processDone
}

func makeClientError(responseErr *RPCError) error {
	if len(responseErr.Data) == 0 {
		return responseErr
	}
	var data struct {
		Kind     string `json:"kind"`
		Expected string `json:"expected"`
		Screen   Screen `json:"screen"`
	}
	if err := json.Unmarshal(responseErr.Data, &data); err != nil || data.Kind == "" {
		return responseErr
	}
	return &WaitError{
		Kind:       data.Kind,
		Expected:   data.Expected,
		LastScreen: data.Screen,
		Cause:      responseErr,
	}
}
