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

## Unsupported sequences

A profile ignores sequences it does not implement without writing their bytes
to the screen, and reports each one as an `unsupported` terminal event whose
data names the sequence in conventional notation, such as `CSI ? 2004 h`,
`CSI 38;2 m`, `ESC # 8`, `OSC 52`, `DCS 1 $ q`, `APC`, or `C1 0x84`.

- A profile emits an event for every occurrence. The session publishes only
  the first occurrence of each distinct sequence, so a host that repeats it on
  every redraw cannot flood event subscribers, and logs it at `Warn` with the
  session and connection identifiers. Repeats are logged at `Debug` and
  counted; `Session.UnsupportedSequences` returns each sequence with the
  number of times it was received. A session tracks at most 256 distinct
  sequences; later new sequences are neither counted nor reported.
- Reports name a mode or SGR parameter individually when the rest of the
  sequence is supported. They never include control-string content, which can
  carry screen text or clipboard data.
- CSI and ESC sequences with an intermediate byte, and CSI sequences with a
  prefix, are reported rather than dispatched by their final byte, so, for
  example, `CSI 1;2 $ r` does not set the scrolling region and `ESC # 8` does
  not restore the cursor. Indirect-color SGR parameters (`38;2;r;g;b` and the
  colon form) consume their components instead of misreading them as
  attributes.

Some sequences are deliberately ignored without a report because they affect
only input TUICast never sends or state a snapshot does not represent: keypad
modes (`ESC =`, `ESC >`), cursor blinking (private mode 12), mouse and focus
reporting (private modes 1000, 1002, 1003, 1004, 1005, 1006, and 1015),
bracketed paste (private mode 2004), and the xterm icon name (`OSC 1`).
Unhandled C0 controls such as NUL padding have no function on a VT220 and are
also ignored silently.

UTF-8 text inside OSC, DCS, PM, APC, and SOS strings does not end the string:
a UTF-8 continuation byte is never read as the 8-bit string terminator `0x9C`.

## VT220 Profile

The initial VT220 profile supports the behavior needed for transcript-driven
automation:

- printable characters and deferred autowrap
- carriage return, line feed, backspace, and horizontal tab
- cursor movement and direct cursor positioning
- display and line erasure
- character insertion, deletion, and erasure (`ICH`, `DCH`, `ECH`)
- line insertion and deletion inside the scrolling region (`IL`, `DL`)
- insert/replace mode (`IRM`, mode `4`)
- autowrap (`DECAWM`, private mode `7`) and cursor visibility (`DECTCEM`,
  private mode `25`)
- vertical scrolling and configurable top/bottom margins
- save and restore cursor, index, next line, reverse index, and reset
- bold, underline, blink, reverse, conceal, and basic ANSI colors
- G0/G1 ASCII and DEC Special Graphics character sets
- resizing while preserving the overlapping upper-left screen region

Unsupported sequences are ignored and reported as described above.
Device-status responses, user-defined keys, downloadable character sets,
double-width or double-height lines, and full VT220 conformance remain outside
this initial profile.

The insert and delete operations are part of the real VT220 and of its terminfo
entry, so applications that see `TERM=vt220` use them for incremental redraws.
Ignoring them leaves the emulated screen out of step with what the application
believes it drew, which an incremental renderer never repairs.

## xterm Profile

The `xterm-256color` profile includes the VT220 behavior above and adds the
subset commonly used by full-screen command-line applications:

- primary and alternate screen buffers (`47`, `1047`, and `1049` modes)
- bright ANSI colors and 256-color indexed foregrounds and backgrounds
- width-two and combining Unicode character cell geometry
- resizing both screen buffers while preserving their overlapping regions
- the window title set with `OSC 0` or `OSC 2`, exposed as the snapshot's
  `Title`; a title change advances the revision like other visible state

The implementation uses the same incremental ANSI parser and emulation engine
as the VT220 profile so fragmented network input behaves consistently. True
color, mouse protocols, configurable tab stops, origin mode, protected cells,
window operations, sixel graphics, and full xterm conformance remain outside
this initial profile. Unsupported sequences are ignored and reported.
