---
title: "How TUICast works"
description: "TUICast's architecture: a driver process that owns SSH and Telnet connections and terminal emulation, thin SDKs that control it, and an optional MCP server."
weight: 10
---

TUICast puts a terminal emulator between your code and a terminal application.
The **driver** connects to the application over SSH or Telnet, emulates the
terminal, and keeps an up-to-date copy of the screen. Your code, through an
**SDK**, tells the driver what to type and what screen to wait for.

```diagram
┌───────────┐  JSON-RPC  ┌──────────────────────┐  SSH or  ┌──────────────┐
│ Your code │   stdio    │ tuicast-driver       │  Telnet  │ Terminal     │
│ + SDK     │◀──────────▶│  connections         │◀────────▶│ application  │
└───────────┘            │  terminal emulation  │          │ (for example │
                         │  screen state, waits │          │  a WMS)      │
┌───────────┐    MCP     ├──────────────────────┤          │              │
│ AI agent  │◀──────────▶│ tuicast-mcp          │◀────────▶│              │
└───────────┘            └──────────────────────┘          └──────────────┘
```

## The Driver

`tuicast-driver` is a single binary that does the hard work: it opens SSH and
Telnet connections, negotiates terminal settings, interprets the escape
sequences the application sends, and maintains each session's screen. It also
implements waiting, because deciding when a screen is ready depends on timing
that only the driver can observe.

An SDK starts its own driver process and talks to it over standard input and
output using [JSON-RPC](../reference/driver-protocol.md). No network port is
opened between your code and the driver, so parallel test workers each get an
isolated driver without port conflicts.

## The SDKs

SDKs for Go, Python, TypeScript, and Java turn the driver's protocol into
ordinary method calls such as `type`, `press`, and `waitForText`. They are
deliberately thin: terminal emulation and timing live in the driver, so every
language behaves the same way. Other languages can use the driver directly
through its [protocol](../reference/driver-protocol.md).

Go programs can also [embed TUICast as a library](../guides/embed-the-go-library.md)
without a separate driver process.

## The MCP Server

`tuicast-mcp` lets AI agents operate terminal applications through the Model
Context Protocol (MCP). It is a separate, optional binary with its own security
boundary: agents can use only connections an operator has approved. See
[Use AI agents with MCP](../guides/use-ai-agents-with-mcp.md).

## Supporting Tools

- The [reference TUI](../getting-started/reference-tui.md) is a practice
  application for learning, demos, and TUICast's own tests.
- **Inspector** is an optional, read-only web page that shows the driver's live
  sessions.
- **Workbench** extends Inspector so you can
  [record a workflow](../guides/record-workflows-with-workbench.md) from a live
  session.

## What TUICast Does NOT Do

- It does not change or instrument the application. It sees only what a user's
  terminal would see.
- It does not support block-mode terminals such as IBM 3270 or 5250.
- It does not decide what your tests check. You choose the screens and text
  that prove a step succeeded.
