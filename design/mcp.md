# TUICast MCP Server

`tuicast-mcp` lets an MCP client observe and operate terminal sessions through
TUICast. It is a separate stdio process from `tuicast-driver`: each MCP process
owns its connections and sessions and closes them when its client disconnects.

## Security Boundary

The model cannot choose a hostname, port, username, credential source, terminal
profile, or screen dimensions. An operator defines these values in named
profiles. MCP tools accept only a profile name, and `tuicast_list_profiles`
returns descriptive metadata without endpoint addresses or credential fields.

SSH passwords and private-key passphrases are read from environment variables.
Private keys and known-hosts data are read from configured files. Secret values
are neither returned by tools nor included in TUICast logs. Host-key validation
is mandatory unless the operator explicitly enables the insecure option, which
emits a warning at startup.

MCP stdio must remain reserved for protocol messages. Logs are written to
standard error.

## Configuration

Start the server with a configuration file:

```sh
TUICAST_WMS_PASSWORD='...' tuicast-mcp -config ./tuicast-mcp.json
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

## Tools

| Tool | Purpose |
| --- | --- |
| `tuicast_list_profiles` | List safe metadata for approved profiles. |
| `tuicast_connect` | Connect to an approved profile. |
| `tuicast_open_session` | Open a terminal with profile-defined settings. |
| `tuicast_screen` | Read the current text screen and cursor. |
| `tuicast_wait_for_text` | Wait up to 120 seconds for exact text. |
| `tuicast_type` | Send literal text without logging it. |
| `tuicast_press` | Send a named or printable key with modifiers. |
| `tuicast_close_session` | Idempotently clean up a session. |
| `tuicast_close_connection` | Idempotently clean up a connection and its sessions. |

Connection and session identifiers are local to one `tuicast-mcp` process.
Closing a connection closes its sessions. The server also closes the full
object graph when the MCP client disconnects.

## Dependency Choice

The server uses the official `github.com/modelcontextprotocol/go-sdk` at
v1.4.0. It supports MCP protocol versions through 2025-11-25 and is the newest
release compatible with TUICast's Go 1.24 baseline. The SDK is transitioning
from MIT to Apache-2.0 licensing; both licenses are compatible with TUICast's
Apache-2.0 license. Later SDK releases require Go 1.25 and should be considered
when TUICast intentionally raises its Go baseline.
