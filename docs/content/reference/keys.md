---
title: "Keys and modifiers"
linkTitle: "Keys"
description: "Named keys and modifiers TUICast can press, including Enter, Tab, arrow keys, F1 to F12, and Control, Shift, and Alt combinations, and how they are encoded."
weight: 40
---

`press` sends a named key, optionally with modifiers. TUICast converts it into
the bytes the session's terminal profile would send. To send literal text, use
`type`; to send arbitrary bytes, use `send`.

## Named keys

| Group | Keys |
| --- | --- |
| Editing | `Enter`, `Tab`, `Backspace`, `Escape`, `Insert`, `Delete` |
| Cursor | `ArrowUp`, `ArrowDown`, `ArrowLeft`, `ArrowRight` |
| Navigation | `Home`, `End`, `PageUp`, `PageDown` |
| Function | `F1` through `F12` |

These are the names used by the driver protocol and MCP. SDKs expose them as
constants in their own style, such as `tuicast.Enter` in Go and
`tuicast.Key.ENTER` in Python.

A single printable character, such as `c`, can also be pressed, which is how
key combinations like Control-C are sent.

## Modifiers

| Modifier | Notes |
| --- | --- |
| `Shift` | Uppercases letters. For shifted punctuation, press the resulting character instead, because it depends on the keyboard layout. |
| `Control` | For example, Control-C. |
| `Alt` | Sent with an Escape prefix. |
| `Meta` | Same as `Alt`. |

Driver protocol example:

```json
{"sessionId": 1, "key": "c", "modifiers": ["Control"]}
```

## Encoding

- Arrow keys follow the cursor-key mode the application selects, normal or
  application mode, as a real terminal does.
- Modified cursor, navigation, and function keys use xterm-compatible
  modifier parameters.
- Shift-Tab sends the standard back-tab sequence.

If an application needs a key that is not listed, send its bytes with `send`.
For example, many VT220 applications use F13 to F20 or keypad keys that
TUICast does not name.
