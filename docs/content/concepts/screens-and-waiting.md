---
title: "Screens and Waiting"
description: "How TUICast keeps an emulated terminal screen, why tests wait for screen content instead of sleeping, and when to use stable and idle waits."
weight: 30
---

TUICast tests read an emulated screen, **not** raw output, and they wait
for that screen to show what they expect instead of pausing for a fixed time.
This page explains why, and how to choose the preferred wait.

## The Screen, Not the Stream

A terminal application does not print lines of output. It sends text mixed
with escape sequences that move the cursor, clear areas, and redraw parts of
the screen. The bytes for one screen can arrive in several pieces, in any
order the application chose.

TUICast applies those bytes to an emulated terminal, the same way a physical
terminal would, and keeps the result: a grid of characters with the cursor
position, colors, and attributes. Your code works with this grid, so
`contains("ORDER SHIPPED")` checks what a user would actually see.

## Screen Snapshots

Reading the screen returns a **snapshot**: a copy of the screen at one moment.
A snapshot never changes, even as the application keeps drawing, so you can
check it at leisure or save it in a test report. Each snapshot has a
revision number that increases when the visible screen changes.

See [Screen snapshots](../reference/screen-snapshots.md) for every field.

## Why Wait Instead of Sleep

SSH and Telnet have no "ready" signal. After you press Enter, the application
may answer in 5 milliseconds or 5 seconds. Fixed sleeps are either too short,
causing random failures, or too long, making tests slow.

A TUICast wait checks the current screen, then rechecks after every update
until a matcher succeeds or the timeout expires. A matcher describes what
the screen should show: text anywhere, an exact line, a cursor position, or a
combination. See [Matchers](../reference/matchers.md).

## Three Kinds of Wait

| Wait | Returns When | Use It When |
| --- | --- | --- |
| Wait | the matcher first succeeds | the expected text appears only when the screen is complete |
| Stable Wait | the matcher succeeds and the host has then been silent for a quiet period | the text appears before the application finishes drawing |
| Idle Wait | the host has been silent for a quiet period | no text reliably identifies the finished screen |

Prefer the first. Use a stable wait when an application draws a screen title
first and fills in fields afterward; otherwise your next keystroke can arrive
before the form is ready. Use an idle wait only as a last resort, because
"silent for a while" is weaker evidence than seeing the right text.

The quiet period restarts whenever the host sends anything, including bytes
that change nothing visible. Typical quiet periods are 50 to 200 milliseconds.

## Choose Strong Matchers

The most reliable matcher identifies the screen uniquely and cannot match the
previous screen:

- Wait for a screen title or field label, not a word that appears on many
  screens.
- Combine conditions, for example a title and the cursor in the first input
  field.
- Wait for a message to disappear, such as `PROCESSING`, with a negated
  matcher.

## When a Wait Fails

A failed wait reports what it expected and the last screen it saw. It also
carries a **flight recording** of the host output that preceded the failure.
See [Diagnose failed waits](../guides/diagnose-failed-waits.md).
