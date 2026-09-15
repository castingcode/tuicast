# Reference application example fixture

The fixture exposes the deterministic reference TUI over SSH on
`127.0.0.1:2222` and Telnet on `127.0.0.1:2323`. The Go SDK launches the
JSON-RPC driver process itself, just as a test suite normally would.

From the repository root, build the binaries and start the fixture:

```sh
mkdir -p /tmp/tuicast-example
go build -o /tmp/tuicast-example/reference-tui ./cmd/reference-tui
go build -o /tmp/tuicast-example/tuicast-driver ./cmd/tuicast-driver
/tmp/tuicast-example/reference-tui --ssh-address 127.0.0.1:2222
```

In a second terminal, start the Telnet fixture:

```sh
/tmp/tuicast-example/reference-tui --telnet-address 127.0.0.1:2323
```

In a third terminal, run the Go examples:

```sh
cd examples/go
TUICAST_DRIVER=/tmp/tuicast-example/tuicast-driver go test -v ./...
```

Run the shared Cucumber features through their Go/Godog bindings:

```sh
cd examples/go
TUICAST_DRIVER=/tmp/tuicast-example/tuicast-driver go run ./cucumber
```

Alternatively, start and stop the reference fixture with Docker Compose:

```sh
./examples/reference/start.sh
TUICAST_DRIVER=/tmp/tuicast-example/tuicast-driver \
  go test -C examples/go -v ./...
./examples/reference/stop.sh
```

Set `TUICAST_REFERENCE_ADDRESS` or `TUICAST_REFERENCE_TELNET_ADDRESS` to use
other fixture addresses. The examples use the reference-only credentials
`operator` / `casting`. Host-key checking is deliberately disabled only for
this local deterministic fixture.
