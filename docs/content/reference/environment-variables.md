---
title: "Environment variables"
description: "Environment variables read by the TUICast reference TUI, MCP server, and install scripts, including MCP secret variables and their _FILE variants."
weight: 20
---

The driver and SDKs read no TUICast-specific environment variables. Connection
settings and credentials are passed in code.

## reference-tui

| Variable | Description |
| --- | --- |
| `TUICAST_REFERENCE_PASSWORD` | SSH password. Required with `-ssh-address` unless `-ssh-users-file` is set. |
| `TUICAST_REFERENCE_APP_PASSWORD` | Password accepted by the application's login screen. Always required. At most 24 characters. |

The examples in the repository read the same two variables to sign in.

## tuicast-mcp

An MCP profile names the variables that hold its secrets, using the
`passwordEnv` and `privateKeyPassphraseEnv` fields. For a name such as
`TUICAST_WMS_PASSWORD`, the server reads:

| Variable | Description |
| --- | --- |
| `NAME` | The secret value. |
| `NAME_FILE` | Path to a file containing the secret, as with Docker and Kubernetes secrets. Cannot be combined with `NAME`. |

If neither is set, the server runs the `-secret-command` executable, if
configured. See [Secrets](mcp-server.md#secrets) for the full rules.

## Install scripts

| Variable | Script | Description |
| --- | --- | --- |
| `BINDIR` | `install.sh` | Installation directory; same as `--bin-dir` |
| `TUICAST_COMPONENT` | both | Component to install; same as `--component` |
| `TUICAST_VERSION` | both | Release to install; same as the version argument |
