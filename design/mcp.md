# TUICast MCP Server

`tuicast-mcp` lets an MCP client observe and operate terminal sessions through
TUICast. It is a separate stdio process from `tuicast-driver`: each MCP process
owns its connections and sessions and closes them when its client disconnects.

## Security Boundary

The model cannot choose a hostname, port, username, credential source, terminal
profile, or screen dimensions. An operator defines these values in named
profiles. MCP tools accept only a profile name, and `tuicast_list_profiles`
returns descriptive metadata without endpoint addresses or credential fields.

A PTY profile runs a local program instead of connecting to a host. The
program, its arguments, working directory, and environment are also fixed by
the profile, and `tuicast_list_profiles` reports only `protocol: "pty"`, never
the command, arguments, or environment. Because such a profile runs a program,
`tuicast-mcp` refuses to start with one unless the operator passes `-allow-pty`;
see [PTY profiles](#pty-profiles).

Profiles name SSH passwords and private-key passphrases rather than containing
them; see [Secrets](#secrets). Private keys and known-hosts data are read from
configured files. Secret values are neither returned by tools nor included in
TUICast logs. Host-key validation is mandatory unless the operator explicitly
enables the insecure option, which emits a warning at startup.

MCP stdio must remain reserved for protocol messages. Logs are written to
standard error.

## Configuration

Start the server with a configuration file:

```sh
TUICAST_WMS_PASSWORD='...' tuicast-mcp -config ./tuicast-mcp.json
```

Or let a command supply secrets that are not in the environment:

```sh
tuicast-mcp -config ./tuicast-mcp.json -secret-command ./wms-secret
```

Example:

```json
{
  "profiles": {
    "warehouse-test": {
      "description": "WMS integration environment",
      "protocol": "ssh",
      "address": "wms-test.example.net:22",
      "terminal": "xterm-256color",
      "width": 100,
      "height": 30,
      "ssh": {
        "username": "automation",
        "passwordEnv": "TUICAST_WMS_PASSWORD",
        "knownHostsFile": "./known_hosts",
        "connectTimeoutMilliseconds": 30000
      }
    },
    "legacy-telnet": {
      "description": "Isolated legacy test host",
      "protocol": "telnet",
      "address": "legacy-test.example.net:23",
      "terminal": "vt220",
      "width": 80,
      "height": 24
    }
  }
}
```

Relative private-key and known-hosts paths are resolved from the configuration
file's directory. The authoritative format is
`schema/mcp-profiles.schema.json`; runtime decoding also rejects unknown fields.

### PTY profiles

A profile with `"protocol": "pty"` runs a program on a pseudo-terminal on the
machine running `tuicast-mcp`, so an agent can explore a local terminal
application without a network service. Each session starts its own instance of
the program, with `TERM` set to the profile's terminal and the PTY sized to the
profile's dimensions. The session ends when the program exits; a non-zero exit
status appears as `sessionError`. Closing the session or connection, or the
client disconnecting, hangs up the program and kills it if it is still running
a second later. PTY profiles work when `tuicast-mcp` runs on Linux or macOS.

```json
{
  "profiles": {
    "rf-client": {
      "description": "Local RF client against the test warehouse",
      "protocol": "pty",
      "terminal": "vt220",
      "width": 40,
      "height": 16,
      "pty": {
        "command": "./bin/rf-client",
        "arguments": ["--warehouse", "DC1"],
        "workingDirectory": "./work",
        "environment": {"WMS_ENV": "test"},
        "environmentFromSecrets": {"WMS_PASSWORD": "TUICAST_WMS_PASSWORD"}
      }
    }
  }
}
```

```sh
tuicast-mcp -config ./tuicast-mcp.json -allow-pty
```

A PTY profile has no `address` or `ssh` settings. A `command` containing a
path separator and a relative `workingDirectory` are resolved from the
configuration file's directory; a bare command name is looked up on the
server's `PATH`. Startup fails if the command cannot be found. The program
inherits the server's environment, without `TERM`, `LINES`, and `COLUMNS`,
plus `environment`, unless `clearEnvironment` is true. Neither map may set
`TERM`, and a variable may appear in only one of them.

`environmentFromSecrets` maps a program environment variable to a secret name,
resolved exactly like `passwordEnv` (see [Secrets](#secrets)). A secret command
runs on every connection attempt. The value is passed only in the program's
environment, which other processes of the same operating-system user can read,
and is never returned by tools or logged.

`-allow-pty` is a flag rather than a profile field for the same reason as
`-secret-command`: only the operator who starts the server can enable it, so a
configuration file from a cloned repository cannot make the server run
programs on its own. With the flag, the server runs exactly the programs its
profiles name, as the server's user; the model still chooses only a profile.
A startup `Info` log records each enabled PTY profile and its command.

## Secrets

`passwordEnv`, `privateKeyPassphraseEnv`, and the values of a PTY profile's
`environmentFromSecrets` name a secret, such as `TUICAST_WMS_PASSWORD`. `tuicast-mcp` resolves each named secret in a fixed
order:

1. The `NAME` environment variable.
2. The file whose path is in the `NAME_FILE` environment variable, following
   the convention of Docker official images. Setting both `NAME` and
   `NAME_FILE` is an error.
3. The executable given by the `-secret-command` flag, run as
   `<command> NAME`. Its standard output is the secret.

Empty variables count as unset. If no source supplies the secret, startup fails
with an error that names all three options. One trailing `\n` or `\r\n` is
removed from file contents and command output; other whitespace is kept because
it can be part of a password. Secrets larger than 64 KiB or empty are rejected.

The order lets a profile be shared while each deployment chooses how to supply
its secrets: environment variables in CI, `_FILE` with Docker or Kubernetes
secrets, and a command that reads a password manager or the operating system's
keychain on a workstation. Only the operator who starts the server chooses the
command; a profile cannot, so a profile from a cloned repository cannot run
programs unless the operator also enables [PTY profiles](#pty-profiles). The
command is a flag rather than a profile field for that reason,
and because MCP clients forward command-line arguments reliably. VS Code's
Agent Host sessions, for example, do not apply `envFile`.

`NAME` and `NAME_FILE` are read at startup. The secret command runs on every
connection attempt instead. It can wait for an unlock prompt without delaying
the MCP handshake, returns rotated credentials without a restart, and keeps the
secret in memory only while authenticating. At startup, `tuicast-mcp` only
checks that the command can be found. The command:

- receives the secret name as its only argument and inherits the server's
  environment;
- reads standard input from the null device, so it cannot consume MCP protocol
  messages, and its standard output is captured, so it cannot corrupt them;
- is stopped after one minute.

A failed command fails the connection. The error and a `Warn` log include the
exit status and the command's standard error, never its standard output. At
startup, an `Info` log records which source supplied each secret.

Example command for 1Password:

```sh
#!/bin/sh
exec op read "op://WMS/$1/password"
```

## Tools

| Tool | Purpose |
| --- | --- |
| `tuicast_list_profiles` | List safe metadata for approved profiles. |
| `tuicast_connect` | Connect to an approved profile. |
| `tuicast_open_session` | Open a terminal with profile-defined settings. |
| `tuicast_screen` | Read the current text screen, cursor, and session state. |
| `tuicast_wait_for_text` | Wait up to 120 seconds for exact text. |
| `tuicast_wait` | Wait for a matcher expression, optionally until stable. |
| `tuicast_wait_for_idle` | Wait until host output has been quiet for a period. |
| `tuicast_wait_for_end` | Wait until the host ends a session, such as after a sign-out. |
| `tuicast_events` | Read bell, enquiry, and unsupported-sequence events. |
| `tuicast_failure_dump` | Read the host output that preceded the session's last failed wait. |
| `tuicast_type` | Send literal text without logging it. |
| `tuicast_press` | Send a named or printable key with modifiers. |
| `tuicast_start_recording` | Start recording a session's successful operations. |
| `tuicast_stop_recording` | Stop recording and return the workflow. |
| `tuicast_close_session` | Idempotently clean up a session. |
| `tuicast_close_connection` | Idempotently clean up a connection and its sessions. |

Connection and session identifiers are local to one `tuicast-mcp` process.
Closing a connection closes its sessions. The server also closes the full
object graph when the MCP client disconnects.

### Parity with the SDKs

A primary use case is an agent exploring a terminal application and then
writing TUICast tests for it. The agent should be able to verify every
expectation it will write, so MCP waits use the same semantics as the driver:

- `tuicast_wait` accepts the driver's recursive matcher expressions
  (`contains`, `line`, `cursor`, `all`, `any`, and `not`) and an optional
  `stableMilliseconds` quiet period, matching `session.wait`.
- `tuicast_wait_for_idle` matches `session.waitForIdle`.
- `tuicast_wait_for_end` matches `session.waitForEnd` and returns the final
  screen. Every screen result also includes `title` when the host has set a
  window title, `sessionState` (`active` or `ended`) and, when a transport or
  terminal failure ended the session, `sessionError`, matching `session.state`.
  An ended session's final screen remains readable until it is closed.
- `tuicast_events` exposes the events an SDK receives from `subscribeEvents`.
  MCP tools cannot push notifications, so the tool returns the session's
  retained events after an `afterSequence` instead. Each session retains its
  most recent 256 events from the moment it opens, so startup events such as
  unsupported-sequence probes are not lost while the agent is busy. `missed`
  counts requested events that are no longer retained. Bells matter for
  automation because RF and warehouse applications commonly beep on errors.
- `tuicast_failure_dump` exposes the flight-recorder dump an SDK receives on a
  failed wait. The service keeps the most recent failed wait of each session.
  The default `readable` format lists each recorded read with Go escapes such
  as `\x1b`, its time offset, and the screen revision it produced, which an
  agent can read directly; `jsonl` returns the same JSON-lines dump the SDKs
  receive.

Both packages decode matchers through `tuicast.MatcherSpec`, so the
expressions cannot drift apart. Because the expressions are recursive, the
`tuicast_wait` input schema is written by hand rather than inferred. MCP wait
durations are capped at 120 seconds; a zero timeout uses the 10-second default.

### Recordings

`tuicast_start_recording` and `tuicast_stop_recording` capture a session's
successful `type`, `press`, and wait operations, including `waitForEnd`, as a
versioned trace in the Workbench recording format described by
`schema/workbench-recording.schema.json`. Screen reads and failed operations
are not recorded, so an agent can explore freely and then record a clean run
of the path it wants to test. Each step maps directly onto an SDK call, giving
the agent a factual basis for generated test code. Recorded timeouts are the
effective values, including defaults.

`tuicast_type` accepts an optional `parameter` name. While recording, the
parameter name is stored instead of the typed text, keeping passwords and other
secrets out of the trace. A session has at most one active recording, and
closing the session discards it.

## Dependency Choice

The server uses the official `github.com/modelcontextprotocol/go-sdk` at
v1.4.0. It supports MCP protocol versions through 2025-11-25 and is the newest
release compatible with the previous Go 1.24 baseline. The SDK is transitioning
from MIT to Apache-2.0 licensing; both licenses are compatible with TUICast's
Apache-2.0 license. Later SDK releases require Go 1.25. The Go baseline is now
1.27.3, so those releases can be considered on a future dependency bump.
