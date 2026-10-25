---
title: "Use AI agents with terminal applications through MCP"
linkTitle: "Use AI agents with MCP"
description: "Let AI agents such as GitHub Copilot or Cursor operate approved terminal applications with the TUICast MCP server, and turn their explorations into tests."
weight: 40
draft: true
---

## Writer brief

**Reader question:** "How do I let an AI agent explore our terminal
application, and can it write TUICast tests for me?"

**Cover:**

- What the MCP server is and why it is a separate install (organizations have
  AI policies).
- The security boundary in two sentences; link
  [Security model](../concepts/security-model.md#ai-agents).
- Step by step: install `tuicast-mcp`, write a profile file, supply the secret,
  register the server in VS Code or Cursor. Base it on `examples/mcp/README.md`
  and `examples/mcp/clients/`.
- The explore-then-record workflow: the agent explores, then records a clean
  run with `tuicast_start_recording` and `tuicast_stop_recording`, then writes
  a test from the recording. Use the prompts in `examples/mcp/prompts/` as
  the worked example.
- Link to the [MCP server reference](../reference/mcp-server.md) for the full
  profile format and tool list rather than repeating it.

**Sources:** `design/mcp.md`, `examples/mcp/`.

**Length:** about 700 words.
