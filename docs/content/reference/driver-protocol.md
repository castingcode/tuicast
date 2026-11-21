---
title: "Driver protocol (JSON-RPC)"
linkTitle: "Driver protocol"
description: "The tuicast-driver JSON-RPC 2.0 protocol over standard input and output: framing, methods, notifications, and error codes, for SDK authors and other languages."
weight: 60
---

`tuicast-driver` accepts JSON-RPC 2.0 requests on standard input and writes
responses and notifications on standard output. Every SDK is built on this
protocol; use it directly to control TUICast from a language without an SDK.
The machine-readable description is the OpenRPC document
[`schema/openrpc.json`](https://github.com/castingcode/tuicast/blob/main/schema/openrpc.json).

## Framing

- Requests use named parameters.
- Input may contain consecutive JSON objects separated by whitespace. Output
  is one JSON object per line.
- Requests run concurrently, so responses can arrive out of order. Match them
  by `id`.
- Standard output carries only protocol messages. Logs go to standard error.
- Closing standard input, or calling `driver.shutdown`, closes everything and
  stops the driver.

```json
{"jsonrpc":"2.0","id":1,"method":"driver.ping"}
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"1"}}
```

## Methods

| Method | Purpose |
| --- | --- |
| `driver.ping` | Return the protocol version. |
| `driver.shutdown` | Close all connections and stop. |
| `connection.open` | Open an SSH or Telnet connection. |
| `connection.close` | Close a connection and its sessions. |
| `session.open` | Open a terminal session on a connection. |
| `session.close` | Close one session. |
| `session.send` | Send UTF-8 `text` or `base64` bytes. |
| `session.press` | Press a [named key](keys.md) with optional modifiers. |
| `session.resize` | Resize the terminal. |
| `session.screen` | Return the current [screen snapshot](screen-snapshots.md). |
| `session.wait` | Wait for a [matcher](matchers.md), optionally until stable. |
| `session.waitForIdle` | Wait until the host has been silent for a quiet period. |
| `session.state` | Report whether a session is `active` or `ended`. |
| `session.waitForEnd` | Wait until the host ends a session. |
| `session.subscribe` | Receive screen updates as notifications. |
| `session.subscribeEvents` | Receive BELL, ENQ, and unsupported-sequence events. |
| `session.unsubscribe` | Cancel a subscription. |

Parameters and results for each method are defined in the OpenRPC document.

## Notifications

After `session.subscribe`, the driver sends `session.screen` notifications,
starting with the current screen. A slow reader may miss intermediate screens
but always receives the newest.

After `session.subscribeEvents`, the driver sends `session.event`
notifications with a `type` of `bell`, `enquiry`, or `unsupported`. The
subscription starts with up to 64 recent events, then continues live without
gaps or duplicates. Event sequence numbers increase within a session.

## Errors

| Code | Meaning |
| --- | --- |
| `-32700` | The request is not valid JSON. |
| `-32600` | The request is not a valid JSON-RPC request. |
| `-32601` | The method does not exist. |
| `-32602` | Invalid parameters, or an unknown connection or session ID. |
| `-32000` | A connection, terminal, wait, or lifecycle failure. |

A failed wait includes structured `data`:

| Field | Description |
| --- | --- |
| `kind` | `timeout`, `canceled`, or `waitFailed` (for example, the session ended). |
| `expected` | Description of what the wait expected. |
| `screen` | The last screen snapshot. |
| `dump` | Flight-recorder dump in JSON-lines format. |
| `dumpPath` | Path of the dump file, when the driver runs with `-dump-dir`. |
