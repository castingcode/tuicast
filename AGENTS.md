# TUICast — Agent Context

This project is a terminal session management and automation server, conceptually similar to how Playwright manages browser instances, pages, and browser sessions.

The primary purpose is to enable automated testing and automation of legacy enterprise TUI (text user interface) applications, particularly warehouse management systems (WMS). These applications commonly expose interactive terminal interfaces over protocols such as SSH and Telnet and may emulate terminal types including VT100, VT220, xterm, and related protocols.

## Core Goals
- Manage many concurrent terminal connections and sessions.
- Connect to one or more remote TUI application servers.
- Support multiple connection protocols, initially including SSH and Telnet.
- It must also be capable of handling telnet negotiation protocol.
- Support multiple terminal emulations/types, including VT220, xterm, and similar ANSI/VT terminal behavior.
- Provide programmatic APIs for creating, controlling, observing, and terminating terminal sessions.
- Make terminal interaction suitable for automated testing, including sending keystrokes/input, waiting for terminal output or screen state, inspecting terminal contents, and asserting expected application behavior.
- Allow multiple independent sessions to interact concurrently with the same application server or with different application servers.
- Provide a reliable abstraction over the underlying connection and terminal protocol so test authors do not need to manage SSH/Telnet connections or terminal state directly.

## Conceptual Model

The system should provide abstractions analogous to Playwright:
- Server — manages the lifecycle of terminal connections and sessions.
- Connection — represents a network connection to a remote terminal server, such as SSH or Telnet.
- Session — represents an interactive terminal session associated with a connection.
- Terminal — maintains terminal state, dimensions, cursor position, screen contents, and terminal-emulation behavior.
- Automation — provides operations for sending input, waiting for output/state changes, inspecting the screen, and performing assertions.

The design should prioritize concurrency, isolation, deterministic behavior, observability, and reliable session lifecycle management.

## Primary Users

The primary users are software engineers and maintainers of enterprise warehouse management systems, especially systems with legacy or terminal-based user interfaces. The project should therefore favor practical integration with existing enterprise environments over assumptions that applications can be modernized or modified.

The project is being developed under the CastingCode GitHub organization. The organization's name is inspired by fly fishing, so fly-fishing terminology may be appropriate for project concepts, examples, or branding when it does not compromise technical clarity.

## Module

```
github.com/castingcode/tuicast
```

Go version: see `go.mod`.

## Project Documentation

Before planning or changing a subsystem, inspect the `design/` directory and
read the documentation relevant to that work. The list below is a guide, not
an exhaustive inventory: documentation may be added without this file being
updated, so do not assume unlisted documents are irrelevant.

- `design/architecture.md` — system design, package boundaries, and dependency
  direction
- `design/session-lifecycle.md` — server, connection, and session ownership and
  lifecycle behavior
- `design/terminal-behavior.md` — terminal contract and supported VT220/xterm
  behavior

Keep implementation and documentation consistent. When behavior or an
architectural contract changes, update the corresponding document as part of
the same work.

Note that the `design` folder is intended for design decisions, ADR documentation,
changelogs, and the like. The `docs` folder is intended for material for someone
who wants to use TUICast.

## Package Structure

```
tuicast/            # Core domain and public orchestration API
  ssh/              # SSH transport adapter
  telnet/           # Telnet transport and negotiation
  ansi/             # Shared ANSI/DEC terminal emulation engine
  vt220/            # VT220 terminal emulation
  xterm/            # xterm terminal emulation
  parsing/          # Terminal byte-stream parsing
  cmd/tuicast/      # Thin binary entry point
```

Use packages to separate independently testable capabilities and enforce
dependency direction. Do not create packages named after architectural roles
such as `ports`, `adapters`, `domain`, `models`, `infrastructure`, or
`internal/core`.

The root `tuicast` package owns session lifecycle, automation, and the primary
consumer-facing API. Protocol and terminal implementations live in
capability-named packages.

Adapter packages may depend on the root package's narrow contracts. The root
domain must not depend directly on concrete adapters. Composition belongs in
the binary or the embedding application.

## Architecture

See `design/architecture.md` for the full design and check `design/` for any newer
or more specific documentation relevant to the work.

**Library first:** The exported API must be suitable for embedding. Keep the
binary as a thin convenience wrapper and composition root.

**Call-site interfaces:** Define interfaces where they are consumed, not where
an implementation happens to live. Keep interfaces narrow and focused on the
methods the consumer actually requires.

**Hexagonal architecture:** Session lifecycle, terminal state, and automation
form the core domain. The core consumes narrow interfaces for external
capabilities and must not depend on SSH, Telnet, a specific terminal emulation,
or another delivery mechanism. Concrete transports, negotiators, parsers, and
terminal emulators plug in at the edges.

Do not create dedicated `ports` or `adapters` packages. Ports are ordinary Go
interfaces defined at their call sites; adapters live in packages named for
their capability or protocol.

**No HTTP framework dependency:** Keep the `Router` contract compatible with
the standard `net/http` handler signature so framework choice remains with the
consumer.

Concrete adapters should include compile-time interface assertions when doing
so does not create an import cycle:

```go
var _ tuicast.Connector = (*Connector)(nil)
```

Keep the assertion alongside the concrete implementation. If an assertion
would create an import cycle, verify compliance in an external contract test.

## Dependency Direction

- The root `tuicast` package contains domain orchestration and declares the
  narrow interfaces it consumes.
- Transport and emulation packages implement those interfaces.
- Adapter packages may depend on `tuicast`; `tuicast` must not depend on
  concrete adapters.
- Adapter packages should not depend on one another unless one capability
  genuinely builds on another, such as terminal profiles depending on `ansi`
  and `parsing`.
- `cmd/tuicast` is the composition root and may import all required packages.

## Dependencies

Introduce dependencies judiciously.

- Prefer the standard library for lifecycle, concurrency, networking, logging,
  and HTTP.
- Reuse mature implementations for complex standards such as SSH and ANSI/VT
  escape-sequence parsing.
- Implement small, project-specific protocol state machines locally when
  available libraries obscure required behavior or introduce disproportionate
  weight.
- Evaluate maintenance status, license, transitive dependency count, API
  stability, and testability before adding a dependency.
- Keep platform-specific and PTY libraries test-only unless runtime behavior
  requires them.
- Do not add frameworks when a narrow interface or standard-library API is
  sufficient.

## Error Handling

- Always wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- The colon-space before `%w` is required
- Do not use bare `errors.New` when you can provide context
- Do not swallow errors silently

## Logging

- Use `log/slog` for all structured logging
- Do not use `log.Default()`, `fmt.Print*`, or `log.Printf`
- Pass loggers via dependency injection; do not use a global logger
- Log at `Debug` for query matching details, `Info` for lifecycle events,
  `Warn` for unexpected-but-handled conditions, `Error` only for genuine failures

## Testing

- Use **GoConvey** (`github.com/smartystreets/goconvey`) for all tests
- Use `Convey` / `So` style — do not mix with `testing.T` assertions
- Table-driven tests are fine but should still use `Convey` blocks per case
- Test files live alongside the code they test (`matcher_test.go`, `session_test.go`)

## What NOT To Do

- Do not define interfaces alongside implementations merely for mocking.
- Do not name packages after architectural roles such as `ports`, `adapters`,
  `domain`, `models`, `infrastructure`, or `internal/core`.
- Do not allow the core domain to depend on concrete transport or terminal
  implementations.
- Do not use `log.Default()` or `fmt.Print*` for logging.
- Do not add dependencies without a concrete need and an assessment of their
  maintenance, stability, licensing, and transitive cost.
