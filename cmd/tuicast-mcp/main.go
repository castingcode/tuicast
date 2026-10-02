package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/mcpserver"
	"github.com/castingcode/tuicast/vt220"
	"github.com/castingcode/tuicast/xterm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Build metadata set by the release build with -ldflags "-X main.version=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, logger); err != nil {
		logger.Error("TUICast MCP server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, input io.ReadCloser, output io.WriteCloser, logger *slog.Logger) error {
	flags := flag.NewFlagSet("tuicast-mcp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "", "path to a TUICast MCP profile configuration file")
	var showHelp, showVersion bool
	flags.BoolVar(&showHelp, "help", false, "show this help and exit")
	flags.BoolVar(&showHelp, "h", false, "shorthand for -help")
	flags.BoolVar(&showVersion, "version", false, "show version information and exit")
	flags.BoolVar(&showVersion, "v", false, "shorthand for -version")
	if err := flags.Parse(arguments); err != nil {
		return fmt.Errorf("parsing TUICast MCP flags: %w", err)
	}
	if showHelp {
		var usage strings.Builder
		usage.WriteString("Usage: tuicast-mcp -config <path> [flags]\n\nFlags:\n")
		flags.SetOutput(&usage)
		flags.PrintDefaults()
		if _, err := io.WriteString(output, usage.String()); err != nil {
			return fmt.Errorf("writing TUICast MCP help: %w", err)
		}
		return nil
	}
	if showVersion {
		if _, err := fmt.Fprintf(output, "tuicast-mcp %s (commit %s, built %s)\n", version, commit, date); err != nil {
			return fmt.Errorf("writing TUICast MCP version: %w", err)
		}
		return nil
	}
	if *configPath == "" {
		return fmt.Errorf("configuring TUICast MCP server: -config is required")
	}
	profiles, err := loadProfiles(*configPath, logger)
	if err != nil {
		return err
	}
	service, err := mcpserver.New(logger, profiles, terminal)
	if err != nil {
		return fmt.Errorf("initializing TUICast MCP service: %w", err)
	}
	protocolServer, err := mcpserver.NewMCPServer(service, version)
	if err != nil {
		return errors.Join(err, service.Close())
	}
	logger.Info("TUICast MCP server ready", "profiles", len(profiles), "version", version, "commit", commit, "date", date)
	runErr := protocolServer.Run(ctx, &mcp.IOTransport{Reader: input, Writer: output})
	return errors.Join(runErr, service.Close())
}

func terminal(profile tuicast.TerminalProfile, width, height int) (tuicast.Terminal, error) {
	switch profile {
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
