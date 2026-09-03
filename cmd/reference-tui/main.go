package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/castingcode/tuicast/reference"
	referencessh "github.com/castingcode/tuicast/reference/ssh"
	referencetelnet "github.com/castingcode/tuicast/reference/telnet"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, logger); err != nil {
		logger.Error("reference TUI stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, input *os.File, output io.Writer, logger *slog.Logger) error {
	flags := flag.NewFlagSet("reference-tui", flag.ContinueOnError)
	flags.SetOutput(output)
	sshAddress := flags.String("ssh-address", "", "listen address for SSH (for example 127.0.0.1:2222)")
	telnetAddress := flags.String("telnet-address", "", "listen address for Telnet (for example 127.0.0.1:2323)")
	sshUsername := flags.String("ssh-username", "operator", "SSH username")
	sshPassword := flags.String("ssh-password", "casting", "SSH password")
	sshHostKey := flags.String("ssh-host-key", "", "PEM-encoded SSH host private key; generated in memory when omitted")
	if err := flags.Parse(arguments); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return fmt.Errorf("parsing reference TUI flags: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("parsing reference TUI flags: unexpected arguments: %v", flags.Args())
	}
	if *sshAddress != "" && *telnetAddress != "" {
		return fmt.Errorf("configuring reference TUI server: --ssh-address and --telnet-address are mutually exclusive")
	}
	if *sshAddress != "" {
		signer, err := hostSigner(*sshHostKey)
		if err != nil {
			return err
		}
		listener, err := net.Listen("tcp", *sshAddress)
		if err != nil {
			return fmt.Errorf("listening for reference SSH connections: %w", err)
		}
		logger.Info("reference TUI SSH server listening", "address", listener.Addr(), "hostKeyFingerprint", gossh.FingerprintSHA256(signer.PublicKey()))
		return referencessh.Serve(ctx, listener, referencessh.Config{
			Username: *sshUsername,
			Password: *sshPassword,
			Signer:   signer,
			Logger:   logger,
		})
	}
	if *telnetAddress != "" {
		listener, err := net.Listen("tcp", *telnetAddress)
		if err != nil {
			return fmt.Errorf("listening for reference Telnet connections: %w", err)
		}
		logger.Info("reference TUI Telnet server listening", "address", listener.Addr())
		return referencetelnet.Serve(ctx, listener, logger)
	}
	return runInteractive(input, output)
}

func runInteractive(input *os.File, output io.Writer) error {
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

func hostSigner(path string) (gossh.Signer, error) {
	if path != "" {
		privateKey, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading SSH host key: %w", err)
		}
		signer, err := gossh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("parsing SSH host key: %w", err)
		}
		return signer, nil
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating SSH host key: %w", err)
	}
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating SSH host signer: %w", err)
	}
	return signer, nil
}
