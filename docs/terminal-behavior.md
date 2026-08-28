# Terminal Behavior

## Contract

A terminal consumes the host's output as an incremental byte stream. Escape
sequences may span any number of writes, so a write ending in a partial
sequence does not change visible state until the sequence is completed.

Terminal profiles may also implement named-key encoding for automation input.
TUICast's VT220 and xterm profiles encode Enter, Tab, Backspace, Escape, cursor
and navigation keys, and function keys F1 through F12. Raw input remains
available when an application requires a sequence outside this set. Arrow-key
encoding follows the host-controlled DEC normal/application cursor-key mode.
Printable keys support Shift, Control, and Alt/Meta combinations. Modified
cursor, navigation, and function keys use xterm-compatible CSI modifier
parameters; Shift-Tab uses the standard back-tab sequence.

BELL and ENQ are non-screen terminal events and do not advance the screen
revision. BELL is observable through the owning session. ENQ is also observable
and causes the session to return the configured printable-ASCII answerback,
which is limited to the VT-compatible maximum of 20 bytes.

Screen coordinates are zero-based with the origin at the upper-left corner.
Snapshots contain cells in row-major order and are detached from future
terminal changes. A snapshot revision advances at most once for each write
that changes visible screen or cursor state, and once for each effective
resize.

Screen text preserves trailing spaces. A normal cell has width one. A wide
character occupies a width-two cell followed by a width-zero continuation
cell; terminal profiles that support wide characters are responsible for
creating that representation.

## VT220 Profile

The initial VT220 profile supports the behavior needed for transcript-driven
automation:

- printable characters and deferred autowrap
- carriage return, line feed, backspace, and horizontal tab
- cursor movement and direct cursor positioning
- display and line erasure
- vertical scrolling and configurable top/bottom margins
- save and restore cursor, index, next line, reverse index, and reset
- bold, underline, blink, reverse, conceal, and basic ANSI colors
- G0/G1 ASCII and DEC Special Graphics character sets
- resizing while preserving the overlapping upper-left screen region

Unsupported sequences are ignored without writing their bytes to the screen.
Device-status responses, user-defined keys, downloadable character sets,
double-width or double-height lines, insert/delete operations, and full VT220
conformance remain outside this initial profile.

## xterm Profile

The `xterm-256color` profile includes the VT220 behavior above and adds the
subset commonly used by full-screen command-line applications:

- primary and alternate screen buffers (`47`, `1047`, and `1049` modes)
- cursor visibility and autowrap private modes
- insert and delete modes for characters and lines
- erase-character operations
- bright ANSI colors and 256-color indexed foregrounds and backgrounds
- width-two and combining Unicode character cell geometry
- resizing both screen buffers while preserving their overlapping regions

The implementation uses the same incremental ANSI parser and emulation engine
as the VT220 profile so fragmented network input behaves consistently. True
color, mouse protocols, configurable tab stops, origin mode, protected cells,
window operations, sixel graphics, and full xterm conformance remain outside
this initial profile. Unsupported sequences are ignored.
