# Getting started with TUICast for Go

TUICast lets Go programs control interactive terminal applications over SSH or
Telnet. The Go SDK launches `tuicast-driver`, which owns the network connection
and terminal emulation.

## Prerequisites

- Go 1.24.5 or newer
- `tuicast-driver` installed and available on `PATH`

On macOS or Linux, install the driver and the reference TUI used by this guide:

```sh
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component all --bin-dir "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
```

See the [main installation instructions](../README.md#driver-and-other-binaries---using-the-install-script)
for Windows and other installation options.

## Install the SDK

Create a Go module and install TUICast:

```sh
mkdir tuicast-go-demo
cd tuicast-go-demo
go mod init example.com/tuicast-go-demo
go get github.com/castingcode/tuicast/sdk/go@v0.0.1
```

## Start the reference TUI

In a separate terminal, start TUICast's deterministic demonstration server:

```sh
reference-tui \
  --ssh-address 127.0.0.1:2222 \
  --ssh-username demo \
  --ssh-password demo-password
```

The SSH transport uses `demo` / `demo-password`. The application displayed
inside the terminal has its own test login: `operator` / `casting`.

## Automate a login

Create `main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	tuicast "github.com/castingcode/tuicast/sdk/go"
)

func main() {
	if err := run(); err != nil {
		slog.Error("TUICast example failed", "error", err)
		os.Exit(1)
	}
	slog.Info("login succeeded")
}

func run() (err error) {
	ctx := context.Background()
	driver, err := tuicast.Launch(ctx)
	if err != nil {
		return fmt.Errorf("launching TUICast driver: %w", err)
	}
	defer func() { err = errors.Join(err, driver.Close()) }()

	connection, err := driver.Connect(ctx, tuicast.SSH{
		Address:                  "127.0.0.1:2222",
		Username:                 "demo",
		Password:                 "demo-password",
		InsecureSkipHostKeyCheck: true, // Safe only for this local test server.
	})
	if err != nil {
		return fmt.Errorf("connecting to reference TUI: %w", err)
	}
	defer func() { err = errors.Join(err, connection.Close(ctx)) }()

	session, err := connection.OpenSession(ctx, tuicast.WithTerminal(tuicast.XTerm256Color))
	if err != nil {
		return fmt.Errorf("opening terminal session: %w", err)
	}
	defer func() { err = errors.Join(err, session.Close(ctx)) }()

	if _, err := session.WaitForText(
		ctx,
		"LOGIN / AUTHENTICATION",
		tuicast.StableFor(50*time.Millisecond),
	); err != nil {
		return fmt.Errorf("waiting for login screen: %w", err)
	}
	if err := session.Type(ctx, "operator"); err != nil {
		return fmt.Errorf("typing user ID: %w", err)
	}
	if err := session.Press(ctx, tuicast.Tab); err != nil {
		return fmt.Errorf("moving to password field: %w", err)
	}
	if err := session.Type(ctx, "casting"); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}
	if err := session.Press(ctx, tuicast.Enter); err != nil {
		return fmt.Errorf("submitting login: %w", err)
	}

	screen, err := session.WaitForText(ctx, "TERMINAL TEST SYSTEM")
	if err != nil {
		return fmt.Errorf("waiting for terminal menu: %w", err)
	}
	if !screen.Contains("Authenticated as operator") {
		return fmt.Errorf("checking authenticated screen: expected operator confirmation")
	}
	return nil
}
```

Format and run the example:

```sh
gofmt -w main.go
go run .
```

`WaitForText` waits against the emulated terminal screen rather than raw
network output. The returned `Screen` is a detached snapshot that can also
inspect lines, cells, colors, attributes, and the cursor. Deferred cleanup
closes the session, connection, and driver and preserves any cleanup errors.

For a real SSH server, replace `InsecureSkipHostKeyCheck` with exactly one
trusted host-key option: `KnownHostsFile` or `HostKeyFingerprint`. Keep
application and SSH credentials in environment variables or a secret manager.
