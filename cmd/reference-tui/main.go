package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/castingcode/tuicast/reference"
	"golang.org/x/term"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(os.Stdin, os.Stdout); err != nil {
		logger.Error("reference TUI stopped", "error", err)
		os.Exit(1)
	}
}

func run(input *os.File, output io.Writer) (runErr error) {
	width, height := 80, 24
	if outputFile, ok := output.(*os.File); ok && term.IsTerminal(int(outputFile.Fd())) {
		terminalWidth, terminalHeight, err := term.GetSize(int(outputFile.Fd()))
		if err != nil {
			return fmt.Errorf("reading terminal dimensions: %w", err)
		}
		width, height = terminalWidth, terminalHeight
	}

	var originalState *term.State
	if term.IsTerminal(int(input.Fd())) {
		state, err := term.MakeRaw(int(input.Fd()))
		if err != nil {
			return fmt.Errorf("enabling raw terminal mode: %w", err)
		}
		originalState = state
		defer func() {
			if err := term.Restore(int(input.Fd()), state); err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("restoring terminal mode: %w", err))
			}
		}()
	}

	application, err := reference.New(width, height)
	if err != nil {
		return fmt.Errorf("creating reference application: %w", err)
	}
	if originalState != nil {
		application.SetVTTestRunner(&terminalVTTestRunner{input: input, state: originalState})
	}
	if err := application.Run(input, output); err != nil {
		return fmt.Errorf("running reference application: %w", err)
	}
	return nil
}

type terminalVTTestRunner struct {
	input *os.File
	state *term.State
}

func (r *terminalVTTestRunner) Run(input io.Reader, output io.Writer) (runErr error) {
	if err := term.Restore(int(r.input.Fd()), r.state); err != nil {
		return &reference.FatalVTTestError{Err: fmt.Errorf("restoring terminal mode for VTTEST: %w", err)}
	}
	defer func() {
		if _, err := term.MakeRaw(int(r.input.Fd())); err != nil {
			runErr = &reference.FatalVTTestError{Err: errors.Join(runErr, fmt.Errorf("restoring reference TUI terminal mode: %w", err))}
		}
	}()
	if err := (reference.CommandVTTestRunner{}).Run(input, output); err != nil {
		return fmt.Errorf("launching VTTEST: %w", err)
	}
	return nil
}
