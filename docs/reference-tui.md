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

The command uses the current terminal dimensions, enters raw mode when attached
to a terminal, and restores both terminal mode and the primary screen on exit.
Use `Ctrl-C` inside the application to exit.

The deterministic test credentials are:

```text
User ID:  operator
Password: casting
```

The login page accepts Tab or arrow keys to change fields, Enter or F1 to log
in, and F2 to clear the form. The authenticated menu supports Up/Down,
Enter/F1, and F2 to sign out. Menu destinations are placeholders except option
12, **ANSI / VT220 Tests**.

## VTTEST

Option 12 launches the authoritative external `vttest` utility from
<https://www.invisible-island.net/vttest/>. Install it with the operating
system package manager (for example, `brew install vttest` on macOS or the
distribution's `vttest` package on Linux) and ensure it is on `PATH`. The large
upstream C source is deliberately not vendored. If the executable cannot be
found or exits unsuccessfully, the menu resumes with a clear `VTTEST
unavailable` diagnostic.

The command leaves its alternate screen and restores cooked terminal mode
before handing the same stdin/stdout terminal to VTTEST. When VTTEST exits, it
re-enters raw mode and redraws the reference menu. VTTEST is interactive and
visual; it does not produce a pass/fail score automatically.

For later scorecard automation, `reference.VTTestRunner` is the observation and
injection boundary. A runner can record selected tests, transcripts, and
ratings while preserving the deterministic suspend/run/resume lifecycle. The
menu exposes stable `Launching VTTEST`, `VTTEST completed`, and `VTTEST
unavailable` states that black-box automation can wait for.

## Design

The `reference` package owns one isolated application session. It incrementally
decodes input, including fragmented UTF-8 and escape sequences, and emits ANSI
output without depending on TUICast itself. This keeps it suitable as an
independent black-box target even though it shares the repository and module.

Rendering deliberately uses the alternate screen, cursor visibility and
positioning, colors, reverse video, and complete screen redraws. Future
scenarios will add controlled partial updates, delayed output, scrolling,
resizing, and other behaviors from the proposal.

The command depends on `golang.org/x/term` solely for portable terminal-size and
raw-mode handling. It is a maintained Go project dependency with a BSD license;
the application state machine and renderer otherwise use the standard library.
