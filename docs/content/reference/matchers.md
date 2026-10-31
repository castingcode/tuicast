---
title: "Matchers"
description: "TUICast matcher expressions for waits: contains, line, cursor, all, any, and not, with their SDK function names and JSON form for the driver and MCP."
weight: 30
---

A matcher describes what a screen must show for a wait to succeed. The same
six matchers are available in every SDK, the driver protocol, and the MCP
server. Rows and columns are zero-based, counted from the upper-left corner.

## Matchers

| Matcher | Succeeds when |
| --- | --- |
| contains *text* | The screen's text contains *text* exactly. |
| line *row*, *text* | Row *row* equals *text* exactly, including trailing spaces. |
| cursor *column*, *row* | The cursor is at that position. |
| all *matchers…* | Every matcher succeeds. An empty list always succeeds. |
| any *matchers…* | At least one matcher succeeds. An empty list never succeeds. |
| not *matcher* | The matcher does not succeed. |

Matching is case-sensitive. The screen's text has a line break between rows,
so *contains* text cannot span two rows unless it includes the line break.

## SDK names

| Matcher | Go | Python | TypeScript | Java |
| --- | --- | --- | --- | --- |
| contains | `Contains` | `contains` | `contains` | `Matcher.contains` |
| line | `LineEquals` | `line_equals` | `lineEquals` | `Matcher.lineEquals` |
| cursor | `CursorAt` | `cursor_at` | `cursorAt` | `Matcher.cursorAt` |
| all | `All` | `all_of` | `all` | `Matcher.all` |
| any | `Any` | `any_of` | `any` | `Matcher.any` |
| not | `Not` | `not_` | `not` | `Matcher.not` |

Each SDK also has a `waitForText` shortcut (`WaitForText` in Go,
`wait_for_text` in Python) equivalent to waiting for *contains*.

## JSON form

The driver's `session.wait` method and the MCP `tuicast_wait` tool take
matchers as JSON. Each object has exactly one key, and expressions can be
nested up to 64 levels:

```json
{
  "all": [
    {"contains": "RECEIVING"},
    {"cursor": {"column": 12, "row": 4}},
    {"not": {"contains": "PROCESSING"}}
  ]
}
```

| Key | Value |
| --- | --- |
| `contains` | string |
| `line` | `{"row": number, "text": string}` |
| `cursor` | `{"column": number, "row": number}` |
| `all` | array of matchers |
| `any` | array of matchers |
| `not` | matcher |

## Wait options

| Option | Effect |
| --- | --- |
| Timeout | Maximum time to wait. Defaults to the SDK's default timeout, 30 seconds unless set when launching the driver. |
| Stable for | After the matcher succeeds, also require this long with no host output, then check again. |

See [Screens and waiting](../concepts/screens-and-waiting.md) for when to use
each.
