---
title: "Security model"
description: "How TUICast handles credentials, SSH host-key verification, captured terminal data, the Inspector and Workbench web UIs, and AI agent access through MCP."
weight: 50
---

TUICast automates systems that often hold business data and require real
credentials. Its defaults keep credentials out of logs, verify the hosts it
connects to, and open no network ports unless you ask.

## Credentials

- SDKs pass passwords and private keys to the driver per connection. The driver
  never writes them to logs or returns them in responses.
- Keep credentials in environment variables or a secret manager, never in test
  code. The examples and the reference TUI follow this rule; the reference TUI
  has no default passwords.
- Text your code types, including passwords typed at an application's login
  screen, is not logged. Recordings store a parameter name instead of the value
  when you mark input as a parameter.

## Host verification

An SSH connection must verify the server's identity in exactly one way:

- a `known_hosts` file;
- the server's expected SHA-256 host-key fingerprint; or
- an explicit opt-out, intended only for local test servers such as the
  reference TUI.

Telnet has no encryption or host verification. Use it only on networks you
trust, typically when the application offers nothing else.

## Captured terminal data

When a wait fails, TUICast keeps the most recent host output, up to 64 KiB, for
diagnosis. That output can contain anything the application displayed,
including business or personal data. TUICast:

- captures what the host sent, not what your code typed;
- writes dump files only when you set a dump directory, and makes them
  readable only by their owner; and
- never includes the contents of unsupported control strings, such as
  clipboard data, in its reports.

Treat test reports that include screens or dumps with the same care as the
application's data.

## No listening ports by default

The driver talks to its SDK over standard input and output. It opens a network
listener only when you enable **Inspector**, its read-only web page. Inspector
has no authentication or TLS, so bind it to `127.0.0.1`; the driver warns when
it is bound to any other address. **Workbench**, which can send input, works
only on loopback addresses and rejects cross-origin requests.

## AI agents

The MCP server limits what an agent can reach. An operator defines named
**profiles** that fix the host, credentials, terminal type, and screen size. An
agent can choose a profile by name but cannot supply an address or read a
credential. Profile listings omit addresses and secrets. Host-key verification
is required unless the operator explicitly disables it, which logs a warning.

Secrets for MCP profiles come from environment variables, files, or a command
that only the operator can configure. See the
[MCP server reference](../reference/mcp-server.md#secrets).

## Reporting a vulnerability

<!-- WRITER: link the project's security policy (SECURITY.md) once it exists. -->
