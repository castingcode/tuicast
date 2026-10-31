---
title: "Terminal support"
description: "Which VT220 and xterm escape sequences and modes TUICast's terminal profiles implement, which are ignored on purpose, and which are not yet supported."
weight: 80
---

This page lists what TUICast's two terminal profiles implement. For how to
choose a profile, see [Terminal emulation](../concepts/terminal-emulation.md).

## vt220

- Printable characters, with deferred autowrap
- Carriage return, line feed, backspace, and horizontal tab
- Cursor movement and direct cursor positioning
- Erasing the display and lines
- Inserting, deleting, and erasing characters (`ICH`, `DCH`, `ECH`)
- Inserting and deleting lines in the scrolling region (`IL`, `DL`)
- Insert and replace mode (`IRM`, mode 4)
- Autowrap (`DECAWM`, mode 7) and cursor visibility (`DECTCEM`, mode 25)
- Scrolling with configurable top and bottom margins
- Save and restore cursor, index, next line, reverse index, and reset
- Bold, underline, blink, reverse, conceal, and the eight basic colors
- G0 and G1 character sets: ASCII and DEC Special Graphics (line drawing)
- Resizing, keeping the overlapping upper-left region
- ENQ answerback, up to 20 printable ASCII characters

## xterm-256color

Everything in `vt220`, plus:

- Primary and alternate screen buffers (modes 47, 1047, and 1049)
- Bright colors and the 256-color palette, for text and background
- Wide (East Asian) and combining Unicode characters
- Window titles (`OSC 0` and `OSC 2`)

## Ignored on purpose

These sequences affect only input TUICast never sends or state a screen
snapshot does not represent. They are ignored without an unsupported-sequence
report:

- Keypad modes (`ESC =`, `ESC >`)
- Cursor blinking (mode 12)
- Mouse and focus reporting (modes 1000, 1002, 1003, 1004, 1005, 1006, 1015)
- Bracketed paste (mode 2004)
- Icon name (`OSC 1`)
- Unused C0 controls such as NUL padding

## Not supported

Everything else is ignored and reported as an unsupported sequence. Notable
gaps:

| Profile | Not supported |
| --- | --- |
| both | Device status and cursor position reports, user-defined keys, downloadable character sets, double-width and double-height lines |
| xterm-256color | True color (24-bit), mouse protocols, configurable tab stops, origin mode, protected cells, window operations, sixel graphics |

Applications that ask for a cursor position report (`CSI 6 n`) and wait for
the answer may stall. If your application depends on a missing feature,
[open an issue](https://github.com/castingcode/tuicast/issues) with a
[flight-recorder dump](../guides/diagnose-failed-waits.md#2-save-the-flight-recorder-dump).
