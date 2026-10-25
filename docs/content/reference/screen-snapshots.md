---
title: "Screen snapshots"
description: "Fields in a TUICast screen snapshot: size, text, cursor, revision, window title, and per-cell characters, colors, and attributes, with their value ranges."
weight: 50
---

A screen snapshot is an unchangeable copy of a session's terminal screen at
one moment. Waits return one, and you can request one at any time. Field names
below are the driver protocol's; SDKs use the same names in their own style.

## Screen

| Field | Type | Description |
| --- | --- | --- |
| `width` | integer | Columns. |
| `height` | integer | Rows. |
| `text` | string | All rows joined with line breaks. Trailing spaces are kept. |
| `cursor` | object | `column`, `row` (zero-based), and `visible`. |
| `revision` | integer | Increases when the visible screen, cursor, size, or title changes. |
| `title` | string | Window title the application last set. Omitted when none; `xterm-256color` only. |
| `cells` | array | `width × height` cells, row by row from the upper-left corner. |

SDKs add helpers such as `contains`, reading a single line, and reading the
cell at a position.

## Cell

| Field | Type | Description |
| --- | --- | --- |
| `text` | string | The character, normally one; may include combining marks. |
| `width` | integer | `1` normally, `2` for a wide character, `0` for the cell after a wide character. |
| `foreground` | integer | Color index; see below. |
| `background` | integer | Color index; see below. |
| `attributes` | integer | Bit flags; see below. |

## Colors

| Value | Meaning |
| --- | --- |
| `-1` | The terminal's default color |
| `0`–`7` | Black, red, green, yellow, blue, magenta, cyan, white |
| `8`–`15` | Bright versions of 0–7 (`xterm-256color`) |
| `16`–`255` | xterm 256-color palette (`xterm-256color`) |

## Attributes

| Bit | Value | Attribute |
| --- | --- | --- |
| 0 | `1` | Bold |
| 1 | `2` | Underline |
| 2 | `4` | Blink |
| 3 | `8` | Reverse video |
| 4 | `16` | Concealed |

For example, `9` means bold and reverse video. Reverse video often marks the
selected menu item or the active field in warehouse applications.
