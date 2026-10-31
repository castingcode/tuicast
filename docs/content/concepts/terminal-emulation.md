---
title: "Terminal emulation"
description: "What TUICast's VT220 and xterm-256color terminal profiles are, how to choose one, how screen size matters, and what happens with unsupported sequences."
weight: 40
---

A **terminal profile** tells TUICast which kind of terminal to imitate.
TUICast offers two: `vt220` and `xterm-256color`. The application adapts its
output to the terminal it believes it is talking to, so the profile must match
what the application expects.

## Choose a Profile

| Profile | Imitates | Choose it For |
| --- | --- | --- |
| `vt220` | DEC VT220 | Legacy and RF warehouse applications; the SDK default |
| `xterm-256color` | Modern xterm | Applications that use colors beyond the basic eight, an alternate screen, Unicode, or window titles |

TUICast reports the profile name to the host as the terminal type (the `TERM`
value), over both SSH and Telnet. That is how the application learns what it
is talking to. To find what your application expects, check how users connect
today: the terminal type configured in their emulator or RF device is usually
the right choice.

## Set the Screen Size

Each session has a width and height in characters; the default is 80 columns
by 24 rows. Many applications lay out screens for a specific size, and RF
handheld applications often use much smaller screens. Use the size your users'
devices use, or screens may wrap or truncate differently than in production.

Your code can resize a session while it runs. TUICast tells the host, and the
application can redraw.

## Keys

Keys such as F1, arrows, and Home send different bytes on different terminals.
TUICast encodes named keys for the session's profile, so `press(F5)` sends what
that terminal would send. Your code can also send raw bytes for anything
unusual. See [Keys](../reference/keys.md).

## Unsupported Sequences

No emulator implements every escape sequence. When the application sends one
TUICast does not support, TUICast ignores it rather than printing garbage, and
reports it as an **unsupported sequence** event, such as `CSI ? 2004 h`. Most
are harmless. If a screen looks wrong or a wait times out, check these events
first; [Diagnose failed waits](../guides/diagnose-failed-waits.md) shows how.

The [terminal support reference](../reference/terminal-support.md) lists what
each profile implements.

## BELL and ENQ

Some control characters do not change the screen:

- **BELL** makes a real terminal beep. Warehouse and RF applications often beep
  on errors, so TUICast reports each bell as an event your test can check.
- **ENQ** asks the terminal to identify itself. TUICast replies with the
  session's configured **answerback** text, as a physical terminal would.
