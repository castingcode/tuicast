---
title: "Recording format"
description: "The versioned, language-neutral JSON format of TUICast recordings produced by Workbench and the MCP server, with each step type and its fields."
weight: 90
draft: true
---

## Writer brief

**Reader question:** "What is in a recording file, and how do I turn it into a
test?"

**Cover:**

- What produces recordings (Workbench and the MCP recording tools) and that
  both use the same format.
- The top-level fields and version, and how parameters keep secrets out of the
  file.
- A table of step types (`waitForText`, `type`, `press`, `wait`,
  `waitForIdle`, `waitForEnd`) with their fields and the SDK call each maps
  to. Note that Workbench produces and replays only `waitForText`, `type`, and
  `press`.
- A short annotated example file.
- A link to the JSON Schema.

**Sources:** `schema/workbench-recording.schema.json`, `design/workbench.md`,
`design/mcp.md` (Recordings).

**Length:** mostly tables; under 500 words.
