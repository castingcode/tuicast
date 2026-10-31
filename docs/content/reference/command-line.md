---
title: "Command-line reference"
linkTitle: "Command line"
description: "Every command-line flag for tuicast-driver, tuicast-mcp, and reference-tui, with defaults, and the options of the TUICast install scripts."
weight: 10
---

Flags can be written with one or two leading dashes, for example `-version` or
`--version`. Every binary accepts `-version` (or `-v`) to print its version,
commit, and build date, and `-h` to print its flags.

## tuicast-driver

```text
tuicast-driver [flags]
```

The driver reads JSON-RPC requests on standard input and writes responses on
standard output. SDKs start it for you. Logs go to standard error.

| Flag | Default | Description |
| --- | --- | --- |
| `-ui-address ADDRESS` | none | Start the read-only Inspector web page on this address, for example `127.0.0.1:0` for any free port. The resolved URL is logged. |
| `-workbench` | off | Enable Workbench on the Inspector. Requires a loopback `-ui-address`. |
| `-dump-dir DIR` | none | Write a flight-recorder dump file to this directory for each failed wait. |

## tuicast-mcp

```text
tuicast-mcp -config PATH [flags]
```

The MCP server communicates with its MCP client over standard input and
output. Logs go to standard error.

| Flag | Default | Description |
| --- | --- | --- |
| `-config PATH` | required | Profile configuration file. See [MCP server](mcp-server.md#configuration). |
| `-secret-command PATH` | none | Executable that prints a named secret when neither `NAME` nor `NAME_FILE` is set. See [Secrets](mcp-server.md#secrets). |

## reference-tui

```text
reference-tui [flags]
```

Without an address flag, the reference TUI runs in the current terminal.
`-ssh-address` and `-telnet-address` cannot be used together.

| Flag | Default | Description |
| --- | --- | --- |
| `-ssh-address ADDRESS` | none | Serve over SSH on this address, for example `127.0.0.1:2222`. |
| `-telnet-address ADDRESS` | none | Serve over Telnet on this address, for example `127.0.0.1:2323`. |
| `-ssh-username NAME` | `operator` | SSH user name. |
| `-ssh-password PASSWORD` | `$TUICAST_REFERENCE_PASSWORD` | SSH password. Prefer the environment variable; command-line values are visible to other local users. |
| `-ssh-users-file PATH` | none | JSON file mapping SSH user names to passwords, such as `{"operator":"...","supervisor":"..."}`. |
| `-ssh-host-key PATH` | generated | PEM-encoded SSH host private key. Without it, a new key is generated at each start. |
| `-app-username NAME` | `operator` | User ID accepted by the application's login screen. At most 24 characters. |
| `-app-password PASSWORD` | `$TUICAST_REFERENCE_APP_PASSWORD` | Password accepted by the login screen. At most 24 characters. Prefer the environment variable. |

## Install scripts

`install.sh` (macOS and Linux) and `install.ps1` (Windows) download a release
from GitHub.

| `install.sh` | `install.ps1` | Default | Description |
| --- | --- | --- | --- |
| `-b`, `--bin-dir DIR` | `-BinDir DIR` | `./bin` | Installation directory |
| `-c`, `--component NAME` | `-Component NAME` | `driver` | `driver`, `mcp`, `reference-tui`, or `all` |
| `VERSION` (last argument) | `-Version VERSION` | latest | Release to install, with or without a leading `v` |

The same settings can be given as environment variables; see
[Environment variables](environment-variables.md).
