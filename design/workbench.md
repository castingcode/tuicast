# TUICast Workbench

TUICast Workbench is an opt-in browser interface for authoring deterministic
terminal workflows from a live driver session. It builds on Inspector screen
streaming but is a separate capability because it can send terminal input.

## Enabling Workbench

Workbench must share an explicitly configured loopback Inspector listener:

```sh
tuicast-driver -ui-address 127.0.0.1:0 -workbench
```

The driver rejects non-loopback Workbench addresses. Inspector remains
read-only when `-workbench` is absent. Workbench mutation endpoints also reject
cross-origin browser requests.

## Recording model

Starting a recording acquires exclusive input control of the selected session.
JSON-RPC clients may continue observing its screens but cannot send input or
resize it until recording stops. Closing the session or its connection always
remains available for lifecycle cleanup.

Workbench records successful operations rather than browser events or raw
terminal bytes:

- `waitForText` records text selected from the current screen;
- `type` records one text input operation or a named parameter; and
- `press` records a named key and optional modifiers.

Users explicitly select assertions because inferred screen changes are often
dynamic or incidental in warehouse applications. Parameterized input sends the
real value to the terminal but stores only its parameter name, keeping secrets
out of downloaded traces.

Recordings use versioned JSON described by
[`schema/workbench-recording.schema.json`](../schema/workbench-recording.schema.json).
Replay acquires exclusive control, executes steps in order, and requires values
for every parameterized input. The first version replays against the original
session; connection setup and cross-session replay belong in a later increment.
