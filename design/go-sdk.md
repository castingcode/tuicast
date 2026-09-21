# Go Driver Client

The Go SDK is an independent module at `sdk/go`:

```go
import tuicast "github.com/castingcode/tuicast/sdk/go"
```

It launches `tuicast-driver` as a child process, verifies protocol compatibility,
correlates concurrent JSON-RPC responses, and owns shutdown. The driver must be
installed on `PATH` or supplied with `tuicast.WithDriverPath`.

## Telnet example

```go
package tui_test

import (
    "context"
    "testing"

    tuicast "github.com/castingcode/tuicast/sdk/go"
)

func TestLogin(t *testing.T) {
    ctx := context.Background()
    client, err := tuicast.Launch(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    connection, err := client.Connect(ctx, tuicast.Telnet{
        Address: "wms.example.test:23",
    })
    if err != nil {
        t.Fatal(err)
    }

    session, err := connection.OpenSession(ctx)
    if err != nil {
        t.Fatal(err)
    }

    screen, err := session.WaitForText(ctx, "User ID:")
    if err != nil {
        t.Fatal(err)
    }
    if position, found := screen.Find("User ID:"); found {
        t.Logf("login prompt starts at column %d, row %d", position.Column, position.Row)
    }

    if err := session.Type(ctx, "operator"); err != nil {
        t.Fatal(err)
    }
    if err := session.Press(ctx, tuicast.Enter); err != nil {
        t.Fatal(err)
    }
    if _, err := session.WaitForText(ctx, "Password:"); err != nil {
        t.Fatal(err)
    }
}
```

The default operation timeout is 30 seconds. An earlier context deadline wins,
and `tuicast.WithDefaultTimeout` changes the client default. Sessions default to
VT220 at 80x24; `WithTerminal` and `WithSize` override those values.

## SSH example

Typed connection configuration rejects missing authentication and ambiguous
host-key verification before contacting the driver:

```go
connection, err := client.Connect(ctx, tuicast.SSH{
    Address:        "wms.example.test:22",
    Username:       "operator",
    PrivateKey:     string(privateKey),
    KnownHostsFile: "/home/test/.ssh/known_hosts",
})
```

## Waiting and screen queries

Simple waits use driver-side matchers and require no explicit timeout:

```go
screen, err := session.WaitForText(ctx, "READY")
screen, err = session.WaitForTextGone(ctx, "LOADING")
screen, err = session.WaitFor(ctx, tuicast.All(
    tuicast.Contains("READY"),
    tuicast.Not(tuicast.Contains("ERROR")),
    tuicast.CursorAt(12, 4),
))
```

`WaitForText` returns on the first matching screen. When a host may still be
rendering, require a quiet period after the match:

```go
screen, err := session.WaitForText(ctx, "READY",
    tuicast.StableFor(100*time.Millisecond),
)
```

When no semantic screen state identifies completion, wait for host output to
be quiet. The default quiet period is 100 milliseconds:

```go
screen, err := session.WaitForIdle(ctx)
screen, err = session.WaitForIdle(ctx,
    tuicast.IdleFor(500*time.Millisecond),
    tuicast.IdleTimeout(10*time.Second),
)
```

Prefer `WaitForText` or a composite matcher when possible. A continuously
updating terminal may never become idle, while a temporarily quiet terminal is
not necessarily ready for the next action.

`Screen` methods are immediate queries over one detached snapshot:

```go
screen, err := session.Screen(ctx)
if screen.Contains("FAILED") {
    t.Fatalf("operation failed:\n%s", screen.Text())
}
position, found := screen.Find("Order 42")
line := screen.Line(4)
cell, ok := screen.CellAt(10, 4)
```

These queries remain client-side so they cannot accidentally inspect a newer
screen through another RPC call. Their language-neutral behavior is recorded
in `schema/testdata/screen-queries.json` and reused by the Java, Python, and
TypeScript SDKs. Timing-sensitive waits remain driver-side so every language
observes the same terminal output and quiet-period semantics.

## Errors and cleanup

Driver-side wait failures become `*tuicast.WaitError`, including the expected
condition and last screen:

```go
screen, err := session.WaitForText(ctx, "READY")
if tuicast.IsTimeout(err) {
    var waitErr *tuicast.WaitError
    if errors.As(err, &waitErr) {
        t.Log(waitErr.LastScreen.Text())
    }
}
```

Closing a session, connection, or client is idempotent. Closing the client asks
the driver to shut down cleanly and terminates the child process if it does not
exit before the configured timeout.

## Design decisions

- The SDK exposes `Driver`, `Connection`, and `Session` rather than mirroring
  raw JSON-RPC methods.
- Typed SSH and Telnet values make invalid configurations harder to express.
- Contexts support caller cancellation, while defaults avoid repeated timeout
  boilerplate.
- Input methods do not automatically wait for idle. A key can legitimately
  produce no output or start a continuous operation, so no universal post-input
  readiness condition exists.
- Matchers and idle detection run in the driver to avoid duplicating concurrent
  terminal synchronization in every language.
- Snapshot queries run locally because they are deterministic, inexpensive,
  and should inspect exactly the snapshot held by the caller.
- The SDK has a separate `go.mod` and no `go.work`; test it independently with
  `GOWORK=off go test ./...` from `sdk/go`.

For a complete GoConvey workflow against the SSH reference application, see
[`examples/go/reference`](../examples/go/reference) and its
[`examples/reference` fixture](../examples/reference/README.md).
