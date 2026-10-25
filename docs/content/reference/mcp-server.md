---
title: "MCP server reference"
linkTitle: "MCP server"
description: "Reference for tuicast-mcp: the profile configuration file, how secrets are resolved from environment variables, files, or a command, and every MCP tool."
weight: 70
---

`tuicast-mcp` exposes TUICast to AI clients through the Model Context Protocol
(MCP) over standard input and output. Each server process owns its own
connections and closes them when the client disconnects. For setup steps, see
[Use AI agents with MCP](../guides/use-ai-agents-with-mcp.md).

## Configuration

```sh
tuicast-mcp -config ./tuicast-mcp.json
```

The configuration file defines named profiles. Agents can only use these
profiles; they cannot supply addresses or credentials.

```json
{
  "profiles": {
    "warehouse-test": {
      "description": "WMS integration environment",
      "protocol": "ssh",
      "address": "wms-test.example.test:22",
      "terminal": "vt220",
      "width": 80,
      "height": 24,
      "ssh": {
        "username": "automation",
        "passwordEnv": "TUICAST_WMS_PASSWORD",
        "knownHostsFile": "./known_hosts"
      }
    }
  }
}
```

### Profile fields

| Field | Required | Description |
| --- | --- | --- |
| `description` | no | Shown to the agent. Describe the environment, not its address. |
| `protocol` | yes | `ssh` or `telnet` |
| `address` | yes | `host:port` |
| `terminal` | yes | `vt220` or `xterm-256color` |
| `width`, `height` | yes | Screen size, 1 to 1000 |
| `ssh` | for SSH | SSH settings; not allowed for Telnet |

### SSH fields

| Field | Description |
| --- | --- |
| `username` | Required. |
| `passwordEnv` | Name of the secret holding the password. |
| `privateKeyFile` | Path to a private key file. |
| `privateKeyPassphraseEnv` | Name of the secret holding the key's passphrase. Requires `privateKeyFile`. |
| `knownHostsFile` | Path to a `known_hosts` file. |
| `hostKeyFingerprint` | Expected SHA-256 host-key fingerprint. |
| `insecureSkipHostKeyCheck` | `true` to skip verification. Logs a warning at startup. |
| `connectTimeoutMilliseconds` | Connection timeout, 0 to 600000. |

Set `passwordEnv`, `privateKeyFile`, or both, and exactly one of
`knownHostsFile`, `hostKeyFingerprint`, or `insecureSkipHostKeyCheck`.
Relative paths are resolved from the configuration file's directory. Unknown
fields are rejected.

The JSON Schema is
[`schema/mcp-profiles.schema.json`](https://github.com/castingcode/tuicast/blob/main/schema/mcp-profiles.schema.json).
Most editors use it for completion and validation.

## Secrets

A profile names its secrets rather than containing them. For a secret named
`TUICAST_WMS_PASSWORD`, the server tries, in order:

1. the `TUICAST_WMS_PASSWORD` environment variable;
2. the file named by the `TUICAST_WMS_PASSWORD_FILE` environment variable;
3. the `-secret-command` executable, run as `COMMAND TUICAST_WMS_PASSWORD`.

| Rule | Detail |
| --- | --- |
| Empty values | Count as unset. |
| Both `NAME` and `NAME_FILE` set | Startup error. |
| Trailing newline | One trailing `\n` or `\r\n` is removed from files and command output. |
| Size | Empty secrets and secrets over 64 KiB are rejected. |
| When read | `NAME` and `NAME_FILE` at startup; the command on every connection attempt. |
| Command limits | Stopped after one minute; no standard input; its output is never logged. |

A secret command lets each workstation read from a password manager. For
example, with 1Password:

```sh
#!/bin/sh
exec op read "op://WMS/$1/password"
```

## Tools

| Tool | Purpose |
| --- | --- |
| `tuicast_list_profiles` | List profile names and descriptions. |
| `tuicast_connect` | Connect to a profile. |
| `tuicast_open_session` | Open a terminal session with the profile's settings. |
| `tuicast_screen` | Read the screen text, cursor, and session state. |
| `tuicast_wait_for_text` | Wait for exact text. |
| `tuicast_wait` | Wait for a [matcher](matchers.md), optionally until stable. |
| `tuicast_wait_for_idle` | Wait until the host has been silent for a quiet period. |
| `tuicast_wait_for_end` | Wait until the host ends the session. |
| `tuicast_events` | Read BELL, ENQ, and unsupported-sequence events. |
| `tuicast_failure_dump` | Read the host output before the session's last failed wait. |
| `tuicast_type` | Type text. The text is not logged. |
| `tuicast_press` | Press a [key](keys.md) with modifiers. |
| `tuicast_start_recording` | Start recording successful operations. |
| `tuicast_stop_recording` | Stop recording and return it in the [recording format](recording-format.md). |
| `tuicast_close_session` | Close a session. |
| `tuicast_close_connection` | Close a connection and its sessions. |

Waits are limited to 120 seconds; a timeout of zero means 10 seconds. Each
session keeps its most recent 256 events.
