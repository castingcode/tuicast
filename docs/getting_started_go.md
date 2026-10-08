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

## Choose credentials

The reference TUI has no default passwords, so choose your own and keep them
out of source code, just as you would for a real WMS environment. Store them in
a `.env.local` file that is excluded from version control:

```sh
cat > .env.local <<'EOF'
TUICAST_REFERENCE_PASSWORD=choose-an-ssh-password
TUICAST_REFERENCE_APP_PASSWORD=choose-an-application-password
EOF
echo .env.local >> .gitignore
```

Load the file into each terminal you use for this guide:

```sh
set -a; . ./.env.local; set +a
```

## Start the reference TUI

In a separate terminal, load `.env.local` and start TUICast's deterministic
demonstration server:

```sh
reference-tui --ssh-address 127.0.0.1:2222
```

Both the SSH transport and the application displayed inside the terminal use
the user ID `operator`. Their passwords are `TUICAST_REFERENCE_PASSWORD` and
`TUICAST_REFERENCE_APP_PASSWORD`, respectively.

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

// requiredEnvironment reads a password from the environment so that it never
// appears in source code.
func requiredEnvironment(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("reading %s: the environment variable is required", name)
	}
	return value, nil
}

func run() (err error) {
	sshPassword, err := requiredEnvironment("TUICAST_REFERENCE_PASSWORD")
	if err != nil {
		return err
	}
	appPassword, err := requiredEnvironment("TUICAST_REFERENCE_APP_PASSWORD")
	if err != nil {
		return err
	}

	ctx := context.Background()
	driver, err := tuicast.Launch(ctx)
	if err != nil {
		return fmt.Errorf("launching TUICast driver: %w", err)
	}
	defer func() { err = errors.Join(err, driver.Close()) }()

	connection, err := driver.Connect(ctx, tuicast.SSH{
		Address:                  "127.0.0.1:2222",
		Username:                 "operator",
		Password:                 sshPassword,
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
	if err := session.Type(ctx, appPassword); err != nil {
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
trusted host-key option: `KnownHostsFile` or `HostKeyFingerprint`. As in this
example, keep application and SSH credentials in environment variables or a
secret manager rather than in source code.
