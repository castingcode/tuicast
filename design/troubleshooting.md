# Troubleshooting Terminal Output

## Problem

Legacy hosts send escape sequences TUICast may not implement. A profile that
ignores or misreads a sequence keeps running, and the first symptom is
usually a wait that times out, often only in CI. Test authors need to answer
three questions without modifying the host application:

1. What did the host send that TUICast ignored?
2. Which bytes produced the screen a failed wait last saw?
3. Does TUICast interpret those bytes the way a real terminal does?

There are two kinds of failure:

- **Unrecognized:** the profile does not implement a sequence. Reporting
  catches this (step 1).
- **Misinterpreted:** the profile recognizes a sequence but handles it
  wrongly. Only replaying the bytes and comparing against another emulator
  catches this (step 4).

## Plan

The work is delivered in four steps. Each builds on the earlier ones:

| Step | Capability | Answers | Status |
| --- | --- | --- | --- |
| 1 | Unhandled-sequence reporting | What was ignored | Implemented |
| 2 | Flight recorder written when a wait fails | Which bytes preceded a failure | Implemented |
| 3 | Full trace capture with runtime toggles | Everything a session received | Planned |
| 4 | Trace replay and comparison with another emulator | Whether TUICast is right | Planned |

## Principles

- **The core owns policy; capability packages own formats.** The root
  `tuicast` package decides when to report, count, or capture, through narrow
  interfaces declared where it uses them. File formats and writers live in a
  capability package such as `trace`, wired up by `cmd/tuicast-driver` or an
  embedding application.
- **Logs carry events; traces carry bytes.** `slog` records lifecycle,
  unsupported sequences, and negotiation. It never records the byte stream,
  which would be too slow and noisy.
- **Capture output, not input, by default.** Host output contains whatever
  the application displays, possibly business or personal data. Input
  contains passwords sent through an SDK's `type` call, which cannot tell
  secrets from other text. Input is captured only on explicit request.
- **Protect captured files.** Trace and dump files are created with mode
  `0600`. MCP tools never return trace contents unless the operator enables
  them.
- **Record bytes exactly as read.** Bugs in sequences split across network
  reads must reproduce, so captures keep read boundaries.

## Step 1: Unhandled-sequence reporting

Every place a profile drops host output reports it in conventional notation,
such as `CSI ? 2004 h`, `CSI 38;2 m`, `OSC 52`, `DCS 1 $ q`, or `APC`. The
reporting contract, including the deliberate ignore list and the rule that
reports never include control-string content, is in
[`terminal-behavior.md`](terminal-behavior.md#unsupported-sequences).

```diagram
┌──────────────┐ every occurrence ┌───────────────────────┐
│ ansi profile │─────────────────▶│ Session               │
└──────────────┘                  │ first: publish, Warn  │
                                  │ repeat: count, Debug  │
                                  └──┬─────────────────┬──┘
                              events │                 │ counts
                                     ▼                 ▼
                 ┌──────────────────────────┐ ┌──────────────────┐
                 │ session.subscribeEvents, │ │ Driver snapshot, │
                 │ SDKs, MCP tuicast_events │ │ Inspector panel  │
                 └──────────────────────────┘ └──────────────────┘
```

- The terminal emits an `EventUnsupported` event for every occurrence. The
  session owns flood control, so every `Terminal` implementation gets the same
  policy.
- The session publishes the first occurrence of each distinct sequence as a
  terminal event, logs it at `Warn` with the session and connection IDs, and
  logs repeats at `Debug`.
- `Session.UnsupportedSequences` returns each distinct sequence with the
  number of times it was received, in order of first appearance. A session
  tracks at most 256 distinct sequences, because parameterized sequences can
  vary without limit.
- Events reach clients through `session.subscribeEvents` (which starts with
  recent retained events), every SDK's event subscription, and the MCP
  `tuicast_events` tool.
- The driver snapshot includes the counts, and the Inspector's live session
  page lists them below the screen.

The xterm profile also records the window title (`OSC 0` and `OSC 2`), and
both profiles report `C1` controls, DCS, PM, APC, and SOS strings.

### Gaps and follow-ups

- **Counts are not queryable on demand.** SDKs and MCP see each sequence
  once, through events, and the counts only in a failure dump (step 2) or the
  Inspector. A JSON-RPC method can follow if users need counts at any time.
- **`Debug` logs need a log-level control.** The driver logs at `Info`, so
  repeat logs are invisible until step 3 adds the runtime log level.
- **Queries go unanswered.** `CSI 6 n` (cursor position report) and other
  queries are reported, but a host may stall waiting for a reply. Answering
  `CSI 6 n` is the first query response worth implementing. It is a terminal
  behavior change, not troubleshooting work.
- **Telnet negotiation is invisible.** The `telnet` package has no logger.
  Negotiation problems need the connection tap in step 3 or a packet capture.

## Step 2: Flight recorder

You cannot turn on tracing after a flaky CI wait has failed. Each session
therefore keeps an always-on flight recorder of recent host output, and every
failed wait carries a copy.

```diagram
┌──────────┐ read ┌───────────────┐ apply ┌──────────┐
│ Stream   │─────▶│ Flight        │──────▶│ Terminal │
└──────────┘      │ recorder ring │       └──────────┘
                  └───────┬───────┘
                          │ copy on failure
                          ▼
                  ┌───────────────┐ dump  ┌─────────────────────────┐
                  │ WaitError     │──────▶│ driver: data.dump,      │
                  │ .Recording    │       │ data.dumpPath (opt-in)  │
                  └───────┬───────┘       │ SDKs: WaitError.dump    │
                          │               └─────────────────────────┘
                          ▼
                  ┌──────────────────────┐
                  │ MCP                  │
                  │ tuicast_failure_dump │
                  └──────────────────────┘
```

- **Tap point:** `Session.readLoop`. Each read is recorded exactly as
  received, after Telnet negotiation and SSH decryption, for every protocol.
  The recorder holds its lock while the terminal applies the read, so any
  screen a caller has observed is already in the recording.
- **Contents:** each read with its offset from session start and the screen
  revision after the terminal applied it, so the last screen of a failed wait
  traces back to the bytes that produced it; resizes in order with the reads;
  and the session's unsupported-sequence counts. Input is not recorded.
- **Bound:** the most recent 64 KiB of output, and at most 4,096 records.
  There is no time window: a 30-second window would capture nothing for a
  30-second wait on a screen that stopped changing, which is exactly the case
  that needs the bytes. The recording reports the dimensions in effect before
  its first record and how many earlier bytes it dropped.
- **Core:** every `WaitError` from a session wait carries a detached
  `FlightRecording`. `Session.FlightRecording` returns one on demand.
- **Driver:** wait failure data always includes `dump`, the recording encoded
  in the trace format below by the `trace` package. With `-dump-dir`, which is
  off by default, the driver also writes the dump to an owner-only
  `tuicast-session-<id>-<random>.jsonl` file and returns `dumpPath`. A dump
  that cannot be written is logged at `Warn` and the wait error is returned
  without `dumpPath`.
- **SDKs:** every SDK's wait error exposes the dump and the optional path
  (`Dump`/`DumpPath` in Go, `dump`/`dump_path` in Python, `dump`/`dumpPath` in
  TypeScript, `dump()`/`dumpPath()` in Java), so a test can attach it to its
  report, write it out, or ignore it.
- **MCP:** the service keeps each session's most recent failed wait, and
  `tuicast_failure_dump` returns it either in a readable form, with bytes
  shown using Go escapes such as `\x1b`, or as the JSON-lines dump.

Decisions:

- The dump is always inline. It is bounded, at most about 400 KB of JSON
  when output arrives in many small reads and closer to 90 KB when it arrives
  in large ones, and the client already receives the screen content it
  describes. Only writing files is opt-in.
- No dump is produced when a session ends on its own outside a wait. A wait
  that fails because the session ended is still a failed wait and carries a
  recording.
- Workbench assertions do not surface dumps yet.

## Step 3: Full trace capture and runtime toggles

A trace records everything a session receives from start to finish, in the
same format as the flight recorder, written to a file as it happens.

The core declares a narrow recorder interface at its call site, along the
lines of `Output(at, data, revision)`, `Input(at, data)`, and
`Resize(at, width, height)`. A `trace` package implements it as a file writer.

Tracing and logging are switched at several scopes:

| Control | Scope | Mechanism |
| --- | --- | --- |
| `-trace-dir` flag or `TUICAST_TRACE_DIR` | Every session in the driver | One trace file per session |
| `trace` option on `session.open` | One session, from an SDK | A test traces only itself |
| `session.startTrace` / `session.stopTrace` | Part of a live session | Capture only around a misbehaving step |
| `-log-level` flag and `driver.setLogLevel` | Driver logging | A `slog.LevelVar` changed at runtime |

Input capture requires a separate explicit option at each scope.

A Telnet connection tap below negotiation is a separate, optional recorder.
It is the only way to see negotiation in-process, because the session tap
starts after it. SSH cannot be tapped below the session because the
connection is encrypted.

## Step 4: Replay and comparison

**Replay.** A command reads a trace or dump, feeds the chunks through a
profile with the same boundaries, and prints the screen at a chosen offset or
revision, or the revision where the screen first matches a matcher. The same
code turns a trace into a regression test, the pattern already used by
`reference/terminal_test.go`.

**Export.** The command can export a trace to asciicast for viewing in
asciinema players. asciicast stores data as UTF-8 text and cannot hold
arbitrary bytes exactly, so it is an export format, not the stored format.

**Comparison.** The command replays the same trace through a reference
emulator and diffs the screens. This is the only way to find misinterpreted
sequences. Candidates are a headless xterm.js, libvterm, or
`tmux capture-pane`. They differ in fidelity, dependencies, and ease of use in
CI, so the choice is deferred to step 4. The reference emulator is a
development tool and must not become a runtime dependency.

Captured traces also make good seed corpora for Go fuzz tests of the parser
and profiles.

## Trace format

Flight-recorder dumps and step 3 traces share one format: one JSON object per
line, so traces stream, append, and survive truncation. The `trace` package
writes version 1:

```json
{"type":"header","version":1,"terminal":"vt220","start":"2026-10-20T19:40:53.726Z","width":80,"height":24,"droppedBytes":0}
{"type":"output","offsetMicroseconds":1520,"revision":1,"base64":"G1s/MTA0OWg="}
{"type":"resize","offsetMicroseconds":9811,"width":132,"height":24}
{"type":"unsupported","sequence":"CSI ? 1049 h","count":1}
{"type":"waitFailure","offsetMicroseconds":2500000,"expected":"screen containing \"READY\"","cause":"context deadline exceeded","screen":{"width":132,"height":24,"revision":1,"cursor":{"column":0,"row":0,"visible":true},"text":"..."}}
```

- The header's `width` and `height` are the dimensions before the first
  record. A non-zero `droppedBytes` means earlier output was not retained, so
  a replay starts mid-stream.
- `output` data is base64 so arbitrary bytes, including invalid UTF-8,
  survive. `revision` is the screen revision after the terminal applied it.
- Offsets are microseconds from session start, avoiding clock skew between
  records.
- `unsupported` records summarize the session's counts at the time of the
  dump. `waitFailure` ends a flight-recorder dump with the failure and its
  last screen.
- Step 3 adds `input` records, written only when input capture is enabled.

## Outside the plan

- **OpenTelemetry.** OpenTelemetry suits fleet trends such as wait latency,
  timeouts, and unsupported-sequence counts across CI runs, but not
  byte-level debugging: span attributes are size-limited and the bytes carry
  business data. The core does not depend on OpenTelemetry. Applications can
  bridge `slog` output to it, and a narrow metrics interface can follow if
  needed.
- **External tools.** `tcpdump` or Wireshark (with its Telnet dissector)
  captures Telnet negotiation and the raw stream today. Running `script` or
  `asciinema rec` around a real `ssh` or `telnet` client gives a reference
  recording to compare against.
- **Terminal type mismatches.** Some `TERM` values advertise capabilities
  TUICast lacks; for example, xterm-256color advertises bracketed paste.
  Choosing a narrower profile often avoids those sequences.
- **Application-specific handlers.** Custom handlers for proprietary
  sequences are deferred until a real case appears.
