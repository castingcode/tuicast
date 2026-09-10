package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/castingcode/tuicast"
	"github.com/castingcode/tuicast/mcpserver"
	"github.com/castingcode/tuicast/vt220"
	"github.com/castingcode/tuicast/xterm"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

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
	if err := flags.Parse(arguments); err != nil {
		return fmt.Errorf("parsing TUICast MCP flags: %w", err)
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
	logger.Info("TUICast MCP server ready", "profiles", len(profiles))
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
