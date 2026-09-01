# Reference TUI

TUICast includes a deterministic terminal application for demonstrations and
integration testing. It lives in the same Go module as the library so API,
emulation behavior, and reference scenarios can change atomically and remain
covered by `go test ./...`.

`docs/proposal-reference-tui.md` is background material only. This document
describes the implemented behavior.

## Running

```sh
go run ./cmd/reference-tui
```

The command uses the current terminal dimensions. Bubble Tea owns raw mode,
input decoding, resize events, alternate-screen rendering, and terminal cleanup.
Use `Ctrl-C` inside the application to exit.

The deterministic test credentials are:

```text
User ID:  operator
Password: casting
```

The login page accepts Tab, Shift-Tab, or arrow keys to change fields, Enter or
F1 to log in, and F2 to clear the form. The authenticated menu supports Up/Down,
Enter/F1, and F2 to sign out. All menu destinations are implemented.

## Forms and Input Fields

Option 2 opens a deterministic warehouse receiving form implemented with
Bubble Tea and Bubbles text inputs. It exercises forward and reverse focus
traversal, editing, required and numeric validation, SKU suggestions, priority
selection, submission, cancellation, and resizing without losing entered
values. Ctrl-N/Ctrl-P select SKU suggestions and Ctrl-Y accepts one.

The fields are purchase order, SKU, quantity, bin location, and multi-line
notes. An accepted receipt displays a stable summary suitable for black-box
assertions.

## Tables

Option 3 opens a Bubbles table containing 30 deterministic warehouse orders.
Arrow keys, Page Up, Page Down, Home, and End navigate the rows. Enter opens the
selected order's detail page, `/` filters by order, customer, status, location,
or item, and `S` cycles sorting through order, status, and quantity.

The table changes its column schema at 76- and 52-column width boundaries. It
preserves the selected order while resizing and retains order, status, and
quantity at the narrowest layout.

## Scrolling

Option 4 contains 100 deterministic event rows with a fixed header. It supports
line, page, first-row, and last-row navigation; follow mode; individual row
appends; and reset. Streaming appends five rows at controlled 75 ms intervals
and ends at the stable `STREAM COMPLETE (5 EVENTS)` state.

## Colors and Attributes

Option 5 renders labeled ANSI 8- and 16-color foreground and background
swatches, selected xterm 256-color cube and grayscale samples, default-color
reset, and bold, underline, blink, reverse, and conceal attributes. It does not
claim unsupported true-color behavior.

## Cursor Movement

Option 6 moves the real terminal cursor with arrow keys and displays the same
zero-based expected column, row, and visibility returned by the driver's
`session.screen.cursor`. Home centers it; F1 and F2 save and restore it; F3
toggles visibility; and F4 runs a deterministic movement script.

## Function Keys and Modifiers

Option 7 records a rolling key history with Bubble Tea's decoded key type,
runes, paste state, and available Shift, Control, and Alt/Meta information. It
accepts function and navigation keys as well as printable control combinations.
The display reports explicit modifiers when available and recognizes F13
through F24 as the legacy Shift-F1 through Shift-F12 convention rather than
treating those values as unrelated keys.

## Partial Screen Updates

Option 8 provides three deterministic timed scenarios. The premature-ready
scenario displays `READY` before the rest of the screen has arrived, ignores
input during that interval, and eventually reaches `SCREEN COMPLETE / INPUT
ENABLED`. It is intended to distinguish a basic text wait from a stable-screen
wait that observes the host-output quiet period.

The independent-regions scenario updates its header, order, counters, and
footer in a deliberately non-sequential order. The in-place progress scenario
repeatedly rewrites a spinner, percentage, progress bar, and status before
reaching `COMPLETE`. Bubble Tea emits these changes as cursor-positioned
differential frames rather than repainting unchanged cells.

## Long-Running Operation

Option 9 provides three lifecycle modes. The finite warehouse wave processes
120 records over approximately ten seconds and reaches `OPERATION COMPLETE`.
The continuous soak mode runs until stopped and bounds its displayed event
history to eight entries. The failure simulation deterministically stops at
60% with error code `E-WAVE-060`.

All modes remain responsive to terminal resize and support pause/resume,
cancellation, and retry where applicable. Their counters, heartbeats, result
states, and failure point are deterministic so automation can make exact
assertions. Unit tests inject tick messages directly rather than waiting for
wall-clock durations.

## Terminal Resize

Option 10 shows actual and target dimensions, resize-event count, responsive
layout breakpoint, and state retained across resizes. `T` toggles the target
between exactly 80x24 and 132x24. Because a server application cannot force its
client-side PTY size, automation presses `T`, calls driver `session.resize` with
the displayed target, and waits for `Status: MATCH`.

## Unicode

Option 11 renders labeled ASCII, combining-mark, CJK, emoji, ZWJ, flag, and
skin-tone samples with expected display widths. Its editable grapheme field is
retained across resize events. Bidirectional terminal behavior is deliberately
outside this scenario.

## VTTEST

Option 12 launches the authoritative external `vttest` utility from
<https://www.invisible-island.net/vttest/>. Install it with the operating
system package manager (for example, `brew install vttest` on macOS or the
distribution's `vttest` package on Linux) and ensure it is on `PATH`. The large
upstream C source is deliberately not vendored. If the executable cannot be
found or exits unsuccessfully, the menu resumes with a clear `VTTEST
unavailable` diagnostic.

Bubble Tea's interactive-command handoff leaves its alternate screen and
restores cooked terminal mode before handing the same stdin/stdout terminal to
VTTEST. When VTTEST exits, Bubble Tea restores raw mode and redraws the menu.
VTTEST is interactive and visual; it does not produce a pass/fail score
automatically.

For later scorecard automation, `reference.VTTestRunner` is the observation and
injection boundary. A runner can record selected tests, transcripts, and
ratings while preserving the deterministic suspend/run/resume lifecycle. The
menu exposes stable `Launching VTTEST`, `VTTEST completed`, and `VTTEST
unavailable` states that black-box automation can wait for.

## Design

The `reference` package owns one isolated Bubble Tea model. Login, menu, forms,
and tables are pages of that model rather than subprocesses, so all implemented
scenarios ship in one reference binary. The application remains an independent
black-box target: it does not depend on TUICast even though it shares the module.

Bubble Tea, Bubbles, and Lip Gloss are maintained, MIT-licensed libraries with
stable versioned APIs. Their real-world renderer and components provide more
representative terminal traffic than a purpose-built renderer alone. The
reference data and workflow remain local and deterministic so automation does
not depend on timing, a network service, or random input.

All scenarios described by the reference menu are now implemented.
