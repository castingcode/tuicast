package reference

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// VTTestRunner runs VTTEST on the same terminal streams as the reference app.
// The narrow contract permits later automation to record invocations/results.
type VTTestRunner interface {
	Run(input io.Reader, output io.Writer) error
}

// FatalVTTestError reports a terminal-lifecycle failure after which the
// reference application cannot safely continue.
type FatalVTTestError struct {
	Err error
}

func (e *FatalVTTestError) Error() string {
	return e.Err.Error()
}

func (e *FatalVTTestError) Unwrap() error {
	return e.Err
}

type vtTestFinishedMsg struct {
	err error
}

type vtTestUnavailableError struct {
	err error
}

func (e *vtTestUnavailableError) Error() string {
	return e.err.Error()
}

func (e *vtTestUnavailableError) Unwrap() error {
	return e.err
}

type vtTestExecCommand struct {
	runner VTTestRunner
	input  io.Reader
	output io.Writer
}

func (c *vtTestExecCommand) Run() error {
	err := c.runner.Run(c.input, c.output)
	if err == nil {
		return nil
	}
	var fatal *FatalVTTestError
	if errors.As(err, &fatal) {
		return fatal
	}
	return &vtTestUnavailableError{err: err}
}

func (c *vtTestExecCommand) SetStdin(input io.Reader) {
	c.input = input
}

func (c *vtTestExecCommand) SetStdout(output io.Writer) {
	c.output = output
}

func (c *vtTestExecCommand) SetStderr(output io.Writer) {
	if c.output == nil {
		c.output = output
	}
}

// CommandVTTestRunner launches the system's vttest executable.
type CommandVTTestRunner struct{}

func (CommandVTTestRunner) Run(input io.Reader, output io.Writer) error {
	path, err := exec.LookPath("vttest")
	if err != nil {
		return fmt.Errorf("finding vttest executable: %w", err)
	}
	command := exec.Command(path)
	command.Stdin = input
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return fmt.Errorf("running vttest: %w", err)
	}
	return nil
}
