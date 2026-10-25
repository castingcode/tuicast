---
title: "Diagnose failed waits"
description: "Find out why a TUICast wait timed out using the last screen, the flight-recorder dump of host output, unsupported-sequence events, and the Inspector."
weight: 20
---

When a TUICast wait times out, the application either never showed what you
expected or showed it in a way TUICast did not interpret correctly. TUICast
gives you three pieces of evidence to tell which: the last screen, a
flight-recorder dump, and unsupported-sequence events.

## 1. Read the last screen

Every SDK raises a **wait error** that includes what the wait expected and the
last screen TUICast saw. Print or attach that screen to your test report.

| What you see | Likely cause | Fix |
| --- | --- | --- |
| A different screen, such as an error message | The application behaved differently | Check test data and the preceding steps |
| The expected screen, but slightly different text | The matcher is too strict | Match a shorter or more stable piece of text |
| The previous screen | The application was slow, or input arrived too early | Increase the timeout, or use a stable wait on the previous step |
| A garbled or partly drawn screen | TUICast misinterpreted the output | Continue to steps 2 and 3 |

The wait error's text ends with the last screen, so test frameworks usually
show it without extra code.

## 2. Save the flight-recorder dump

Each session keeps the most recent host output, up to 64 KiB, exactly as
received. A failed wait includes it as a **dump** in the wait error's `dump`
field: the bytes, when each read arrived, which screen revision it produced,
and which sequences TUICast ignored.

Save the dump as a test artifact so failures in CI can be studied later. To
have the driver write a file for every failed wait instead, start it with a
dump directory:

```sh
tuicast-driver -dump-dir ./tuicast-dumps
```

SDKs pass driver flags through their launch options, such as `WithDriverArgs`
in Go. The wait error's `dumpPath` field then names the file.

Dumps contain whatever the application displayed. Store them as carefully as
the application's data.

## 3. Check unsupported sequences

If the screen looks wrong, TUICast may have ignored a sequence the application
relies on. Subscribe to the session's events, or read the dump, and look for
`unsupported` entries such as `CSI ? 2004 h`.

Many unsupported sequences are harmless, such as requests to enable mouse
reporting. Suspect one when it appears just before the screen goes wrong. Check
the [terminal support reference](../reference/terminal-support.md), and try the
other terminal profile if your application supports it.

If you believe TUICast handles a sequence incorrectly,
[open an issue](https://github.com/castingcode/tuicast/issues) and attach the
dump.

## 4. Watch the session live

To watch a session while a test runs, start the driver with the Inspector:

```sh
tuicast-driver -ui-address 127.0.0.1:0
```

The driver logs the Inspector's URL to standard error. Its session page shows
the live screen and the unsupported sequences received so far.

## Common causes

- **Timeout too short for a slow step.** Long-running operations, such as
  allocating a wave, need a longer timeout on that wait only.
- **Typing before the screen is ready.** Use a
  [stable wait](../concepts/screens-and-waiting.md#three-kinds-of-wait) on the
  screen before the input.
- **Wrong terminal size.** The application wrapped or truncated text
  differently. See [Terminal emulation](../concepts/terminal-emulation.md).
- **Text split across lines.** A `contains` matcher checks the screen's text
  with a line break between rows, so text that wraps onto the next row does
  not match as one string.
