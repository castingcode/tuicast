# Driver Web UI Design

## Goals

The driver should expose a small, read-only operational UI that:

- lists active connections and sessions;
- shows connection ID, protocol, remote address, session ID, terminal profile,
  dimensions, screen revision, and lifecycle/error state;
- links each session to a live terminal screen;
- listens on an ephemeral loopback port by default and permits an explicit
  address or port; and
- preserves stdout exclusively for JSON-RPC protocol traffic.

The first version should not send terminal input, display credentials, or be
reachable off-host by default.

## Command-line contract

Add a `-ui-address` flag whose default is `127.0.0.1:0`, so the UI is available
without configuration. Port zero asks the operating system for an available
ephemeral port. An operator can choose a stable port with, for example:

```sh
tuicast-driver -ui-address 127.0.0.1:8080
```

An empty `-ui-address` disables the UI for embedders or constrained execution
environments that do not want an additional listener.

After calling `net.Listen`, log the resolved URL through the injected
`slog.Logger` at Info level. The command's logger writes to stderr, avoiding
corruption of JSON-RPC stdout. Explicit non-loopback addresses are allowed but
should log a warning because the initial UI has no authentication or TLS.

## Package and ownership

Put the HTTP implementation in `driverui`, not in the root domain or the
transport adapters. `cmd/tuicast-driver` remains the composition root: it
creates the driver, creates the UI with a narrow snapshot source supplied by
the driver, starts both, and shuts the HTTP server down when either stdin closes
or `driver.shutdown` succeeds.

```diagram
┌────────────────────┐  read-only snapshots  ┌───────────────┐
│ driverui HTTP/SSE  │◀──────────────────────│ driver.Server │
└─────────┬──────────┘                       └───────┬───────┘
          │                                          │
          ▼                                          ▼
┌────────────────────┐                       ┌───────────────┐
│ Browser (embedded  │                       │ Connections / │
│ HTML, CSS, and JS) │                       │ sessions      │
└────────────────────┘                       └───────────────┘
```

The driver registry should retain only UI-safe metadata when objects are
opened: protocol and address for a connection; terminal profile for a session.
It must not retain or expose passwords, private keys, passphrases, usernames,
or host-key configuration. Snapshot methods copy metadata while holding the
registry mutex and call session screen/error methods only after releasing it.
This keeps HTTP clients from holding the lifecycle lock.

Use a call-site interface in `driverui` resembling:

```go
type SnapshotSource interface {
    DriverSnapshot() driver.Snapshot
    SessionScreens(context.Context, uint64) (<-chan tuicast.Screen, error)
}
```

The exact exported DTO names can be settled during implementation, but they
should be immutable values with JSON tags rather than exposing registry
records or live domain objects.

## HTTP surface

Use `http.ServeMux` and `http.Server` directly:

| Route | Behavior |
| --- | --- |
| `GET /` | Embedded connection/session index. |
| `GET /sessions/{id}` | Embedded live screen page. |
| `GET /api/state` | Current detached driver snapshot as JSON. |
| `GET /api/sessions/{id}/screens` | Screen updates as server-sent events. |
| `GET /healthz` | Liveness response for local/container tooling. |

Return 404 for an unknown or closed session. Set `Cache-Control: no-store` on
API and HTML responses, enforce GET-only handlers, configure read-header and
idle timeouts, and use `http.Server.Shutdown` with a bounded context.

SSE is preferable to WebSockets here: updates flow only from driver to browser,
it is supported by `net/http` and `EventSource`, and it requires no dependency
or custom framing. Each event contains the same screen representation as the
JSON-RPC `session.screen` notification. A slow browser naturally benefits from
the session's existing coalesced `Screens` stream. Send periodic SSE comments
to detect disconnected clients when the screen is idle.

## Frontend

Embed one HTML template, one stylesheet, and one small JavaScript module with
`//go:embed`. Use semantic tables and links on the index. The screen page uses
a `<pre>` for the text snapshot initially; a later enhancement can render the
cell array for colors and attributes without changing the endpoint. JavaScript
uses `fetch` for state and `EventSource` for live updates. No Node.js,
framework, generated assets, or frontend build step is warranted.

Poll `/api/state` every two seconds on the index so connections and sessions
appear and disappear. The live page should show reconnecting/disconnected
state using native `EventSource` callbacks and update dimensions, cursor, and
revision beside the screen.

## Verification plan

Use `httptest` and GoConvey to cover the index, JSON snapshot, unknown session,
method rejection, and the first/current SSE screen event. Add a command-level
test that binds `127.0.0.1:0`, confirms the logged address contains a resolved
nonzero port, and verifies shutdown closes the listener. Run the race detector
against driver and UI tests because registry closure and SSE observation occur
concurrently.
