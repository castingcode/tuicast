## Design Constraints

- **Library first.** The exported API surface should be usable for embedding; the
  binary is a thin convenience wrapper.
- **Call-site interfaces.** Define interfaces in the consuming package, not
  alongside the struct that satisfies them. Interfaces should be narrow — only
  the methods the consuming package actually uses.
- **No HTTP framework dependency.** The `Router` interface accepts any router
  compatible with the standard `net/http` handler signature, keeping framework
  choice with the consumer.

## Hexagonal Architecture

TUICast uses [hexagonal architecture](https://alistair.cockburn.us/hexagonal-architecture/)
(also known as ports and adapters). The core domain — session lifecycle, terminal
state, and automation — does not depend on SSH, Telnet, a specific terminal
emulation, or any other delivery mechanism. Those concerns plug in at the edges.

**Ports** are the contracts the core needs in order to talk to the outside world:
open a connection, read and write a byte stream, parse a terminal protocol, and
so on. In Go these are ordinary interfaces, defined where they are consumed (see
call-site interfaces above), not in a dedicated "ports" package.

**Adapters** are the concrete implementations of those ports: an SSH client, a
Telnet negotiator, a VT220 emulator, and similar. The core depends on the port;
an adapter satisfies it. Swapping Telnet for SSH, or VT220 for xterm, should not
require changing domain logic.

This keeps transport, emulation, and automation independently testable and lets
new protocols or terminal types be added without rewriting the session model.

## Language-Neutral Driver

The root package remains the embeddable Go API. The `driver` package exposes
that API as JSON-RPC 2.0 over streams, while `cmd/tuicast-driver` composes the
concrete SSH, Telnet, VT220, and xterm implementations. This preserves the core
dependency direction: the driver depends on the domain and adapters, but the
domain does not depend on the driver or a delivery protocol.

The initial executable communicates over standard input and output so each test
worker can own an isolated process without port allocation or authentication.
Language SDKs should be thin clients over the versioned protocol rather than
reimplementing terminal state or session lifecycle.

## Package Naming

Do not name packages after architectural roles. Names such as `ports`,
`adapters`, `models`, `domain`, `infrastructure`, or `internal/core` describe
the pattern, not the code.

Name packages after what they actually do. Prefer capability and protocol
language over layer language, for example:

- `vt220` — VT220 terminal emulation
- `xterm` — xterm terminal emulation
- `ansi` — shared ANSI/DEC terminal emulation behavior
- `ssh` — SSH transport
- `telnet` — Telnet connection and option negotiation
- `parsing` — parse terminal byte streams into structured updates

A package named `vt220` tells a reader what lives there. A package named
`adapters` does not.
