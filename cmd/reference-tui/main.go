package main

import (
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

func run(input *os.File, output io.Writer) error {
	width, height := 80, 24
	if outputFile, ok := output.(*os.File); ok && term.IsTerminal(int(outputFile.Fd())) {
		terminalWidth, terminalHeight, err := term.GetSize(int(outputFile.Fd()))
		if err != nil {
			return fmt.Errorf("reading terminal dimensions: %w", err)
		}
		width, height = terminalWidth, terminalHeight
	}

	application, err := reference.New(width, height)
	if err != nil {
		return fmt.Errorf("creating reference application: %w", err)
	}
	if err := application.Run(input, output); err != nil {
		return fmt.Errorf("running reference application: %w", err)
	}
	return nil
}
