# Session Lifecycle

TUICast separates orchestration from transport-specific connection topology:

```text
Server -> Connection -> Session -> Terminal
             |             |
         SessionOpener  SessionStream
```

- `Server` assigns identifiers, tracks connections, and closes the complete
  object graph.
- `Connection` wraps one transport connection and tracks its sessions.
- `Session` owns the stream-to-terminal read loop and exposes input, resizing,
  screen snapshots, and cancellable waits.
- Transport packages implement `Connector`, `SessionOpener`, and
  `SessionStream`; the root package does not import them.

Closing is idempotent and proceeds from sessions to connections. Closing a
session stream must unblock any pending read. A remote end-of-file completes a
session without treating it as a failure, while transport and terminal errors
are available through `Session.Err`.

## Transport Topology

An SSH `Connection` is one authenticated TCP connection. Each TUICast session
is a separate SSH channel, so multiple interactive sessions share the same
authentication and network connection. OpenSSH commonly limits this to ten
channels through `MaxSessions`. Applications needing hundreds of sessions can
establish multiple TUICast connections with the same SSH connector and spread
sessions across them without creating a separate `Server`.

A Telnet `Connection` owns one TCP connection and permits one session because
Telnet does not provide SSH-style channel multiplexing. The Telnet stream
handles IAC escaping, option negotiation, terminal-type reporting, and NAWS
window-size updates before terminal bytes reach the emulator.

The `memory` package provides the same contracts over `net.Pipe`. It is useful
for deterministic automation tests that should exercise the real session
lifecycle without a network service.

## Waiting for Screen State

`Session.WaitFor` checks the current snapshot before sleeping, then wakes after
each terminal update or resize. This avoids missing an update between checking
the screen and subscribing for changes. A wait ends when its matcher succeeds,
its context is canceled, or the session ends.

```go
screen, err := session.WaitFor(ctx, tuicast.ScreenContains("READY"))
```

`WaitFor` returns as soon as one snapshot matches. When a host may still be
rendering after the expected text first appears, `WaitForStable` additionally
requires a period with no host output and checks the matcher again afterward:

```go
screen, err := session.WaitForStable(
    ctx,
    tuicast.AllOf(
        tuicast.ScreenContains("READY"),
        tuicast.CursorAt(12, 4),
    ),
    100*time.Millisecond,
)
```

The quiet period resets for every byte read from the host, including bytes that
do not produce a visible screen revision. This is a synchronization heuristic:
SSH and Telnet do not expose an application-ready signal, so callers should use
the strongest application-specific matcher available and choose a suitable
quiet period.

Canceled and failed waits return a `WaitError`. It retains the matcher
description and last detached snapshot, allowing assertion libraries to report
the actual screen without TUICast depending on a test framework.

Snapshots are detached from later updates and may be safely retained by test
code while the session continues processing output.

## Input and Observation

`Session.Send` writes raw input. `Session.Press` encodes named keys such as
`KeyEnter`, arrow keys, and `KeyF1` through `KeyF12` according to the terminal
profile:

```go
if err := session.Press(tuicast.KeyEnter); err != nil {
    return err
}
```

`Press` also accepts `ModifierShift`, `ModifierControl`, and `ModifierAlt`
(`ModifierMeta` is an alias). A `Key` containing one printable character can be
used for combinations such as Control-C. Named cursor, navigation, and function
keys use xterm-compatible CSI modifier parameters.

`Session.Screens` provides a cancellable stream of detached snapshots. It emits
the current screen first and then visible revisions. The stream is buffered and
coalesces updates, so a slow observer receives the newest available state
rather than blocking terminal processing:

```go
for screen := range session.Screens(ctx) {
    observe(screen)
}
```

`Session.Events` separately observes non-screen controls such as BELL and ENQ.
Events carry session-monotonic sequence numbers. ENQ causes the session to send
the terminal's configured answerback before publishing the event. Event
subscribers are buffered; a subscriber that cannot keep up is closed rather
than blocking terminal output processing.
