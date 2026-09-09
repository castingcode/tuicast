# TUICast Driver Protocol

`tuicast-driver` exposes the Go library to other programming languages through
JSON-RPC 2.0 over standard input and standard output. A language SDK can launch
one driver process per test worker and map its native API onto these methods.

The driver and Go library remain in the same module. The `driver` package owns
the protocol and object registry; `cmd/tuicast-driver` is the composition root
for SSH, Telnet, VT220, and xterm implementations.

The machine-readable OpenRPC description is available at
[`schema/openrpc.json`](../schema/openrpc.json). It uses named parameters, as
does the driver's JSON-RPC interface.

## Running

```sh
go run ./cmd/tuicast-driver
```

Standard output is reserved for JSON-RPC responses and notifications. Logs are
written to standard error. Input may contain consecutive JSON objects separated
by whitespace; output contains one JSON object per line. Requests execute
concurrently, so responses may arrive out of order and must be correlated by
`id`.

```json
{"jsonrpc":"2.0","id":1,"method":"driver.ping"}
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"1"}}
```

Closing standard input or calling `driver.shutdown` closes all subscriptions,
sessions, and connections.

## Methods

| Method | Purpose |
| --- | --- |
| `driver.ping` | Return the driver protocol version. |
| `driver.shutdown` | Close the object graph and stop the process. |
| `connection.open` | Open an SSH or Telnet connection. |
| `connection.close` | Close a connection and all of its sessions. |
| `session.open` | Open a terminal session on a connection. |
| `session.close` | Close one session. |
| `session.send` | Send UTF-8 text or base64-encoded bytes. |
| `session.press` | Send a named terminal key. |
| `session.resize` | Resize the remote and emulated terminals. |
| `session.screen` | Return the current detached screen snapshot. |
| `session.wait` | Wait for a serializable matcher, optionally until stable. |
| `session.waitForIdle` | Wait until host output has been quiet for a period. |
| `session.subscribe` | Subscribe to coalesced screen revisions. |
| `session.subscribeEvents` | Subscribe to BELL and ENQ terminal events. |
| `session.unsubscribe` | Cancel a screen or event subscription. |

### Connections

Telnet requires `protocol` and `address`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "connection.open",
  "params": {
    "protocol": "telnet",
    "address": "wms.example.test:23",
    "connectTimeoutMilliseconds": 10000
  }
}
```

SSH additionally requires `username` and at least one of `password` or
`privateKey`. An encrypted private key uses `privateKeyPassphrase`. Host-key
verification requires exactly one of:

- `knownHostsFile`: path on the driver machine;
- `hostKeyFingerprint`: expected SHA-256 fingerprint;
- `insecureSkipHostKeyCheck`: explicit opt-out intended only for controlled
  test environments.

Passwords, keys, and passphrases are accepted only in requests and are never
included in lifecycle logs or responses.

### Sessions and input

`session.open` accepts `connectionId`, terminal profile (`vt220` or
`xterm-256color`), `width`, and `height`. Its optional printable-ASCII
`answerback` (up to 20 bytes) is sent automatically when the host transmits
ENQ.

`session.send` requires exactly one of `text` or `base64`:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "session.send",
  "params": {"sessionId": 1, "text": "operator"}
}
```

Named keys use the values exported by the Go API: `Enter`, `Tab`, `Backspace`,
`Escape`, `ArrowUp`, `ArrowDown`, `ArrowRight`, `ArrowLeft`, `Home`, `End`,
`Insert`, `Delete`, `PageUp`, `PageDown`, and `F1` through `F12`.

`session.press` also accepts a `modifiers` array containing `Shift`, `Control`,
`Alt`, or `Meta`. Alt and Meta are aliases under conventional ANSI encoding.
One printable character may be used as the key, allowing combinations such as
`{"key":"c","modifiers":["Control"]}`. Shift uppercases letters; callers
should supply the resulting character for keyboard-layout-dependent shifted
punctuation.

### Waiting

`session.wait` requires a positive `timeoutMilliseconds`. A positive
`stableMilliseconds` uses TUICast's host-output quiet-period behavior; omitting
it returns on the first matching snapshot.

Matchers are recursive JSON expressions. Exactly one expression is allowed at
each level:

```json
{
  "all": [
    {"contains": "READY"},
    {"cursor": {"column": 12, "row": 4}},
    {"not": {"contains": "LOADING"}}
  ]
}
```

Available expressions are `contains`, `line` (`row` and exact `text`),
`cursor`, `all`, `any`, and `not`.

`session.waitForIdle` requires positive `timeoutMilliseconds` and
`quietMilliseconds` values. Its quiet period resets for every host byte,
including terminal controls that do not visibly change the screen. It returns
the screen captured at the end of the quiet period.

### Screens and subscriptions

Screen results contain `width`, `height`, `revision`, `text`, `cursor`, and the
row-major `cells` array. Colors use TUICast palette indexes, with `-1` meaning
the terminal default. Attributes are the bitmask defined by the Go `Attributes`
type.

`session.subscribe` returns a `subscriptionId`. The driver then emits JSON-RPC
notifications, beginning with the current snapshot:

```json
{
  "jsonrpc": "2.0",
  "method": "session.screen",
  "params": {
    "subscriptionId": 1,
    "sessionId": 1,
    "screen": {"width": 80, "height": 24, "revision": 4, "text": "..."}
  }
}
```

Slow consumers may skip intermediate revisions but receive the newest available
screen.

`session.subscribeEvents` uses the same subscription lifecycle and emits
`session.event` notifications. Event types are `bell` and `enquiry`; enquiry
events include the configured answerback in `data`. Event sequence numbers are
monotonic within a session. The driver sends the answerback to the host before
publishing the enquiry event.

The Go SDK exposes these methods through `Session.Subscribe` and
`Session.SubscribeEvents`. Screen subscriptions coalesce unread revisions.
Event subscriptions preserve order in a bounded buffer and close their event
channel if the consumer cannot keep up, preventing notifications from blocking
unrelated JSON-RPC responses. Both subscription types have an idempotent
`Close` method that calls `session.unsubscribe`.

## Errors

The driver uses standard JSON-RPC error codes for malformed requests:

- `-32700`: parse error
- `-32600`: invalid request
- `-32601`: unknown method
- `-32602`: invalid parameters or unknown object identifier
- `-32000`: transport, terminal, wait, or lifecycle failure

Application error messages retain TUICast's operation context and wait
diagnostics. Wait failures also include structured JSON-RPC error `data` with
`kind`, `expected`, and the last `screen`, allowing clients to report useful
diagnostics without parsing the human-readable message.
