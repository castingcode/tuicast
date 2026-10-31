---
title: "TUICast documentation"
linkTitle: "Documentation"
description: "TUICast automates and tests interactive terminal applications over SSH and Telnet, with VT220 and xterm emulation and SDKs for Go, Python, TypeScript, and Java."
cascade:
  type: docs
---

TUICast is an open-source tool for automating and testing interactive terminal
applications, such as warehouse management systems (WMS) used from RF
terminals. It connects over SSH or Telnet, emulates a VT220 or xterm terminal,
and lets your code type, press keys, wait for screens, and check what the
application displayed.

Think of it as Playwright for terminal applications: your tests describe what a
user does and sees, and TUICast handles the connection, terminal emulation, and
timing.

## Start here

- **New to TUICast?** [Install it](getting-started/installation.md), then
  automate your first login in [Go](getting-started/go.md),
  [Python](getting-started/python.md),
  [TypeScript](getting-started/typescript.md), or
  [Java](getting-started/java.md).
- **Deciding whether it fits?** Read
  [How TUICast works](concepts/how-tuicast-works.md) and the [FAQ](faq.md).
- **Connecting to your own system?** Follow
  [Connect to your application](guides/connect-to-your-application.md).
- **A wait timed out?** See [Diagnose failed waits](guides/diagnose-failed-waits.md).
- **Looking up a flag, key, or method?** Go to the [Reference](reference/).

## What TUICast does

- Opens many isolated terminal sessions at once, over SSH or Telnet.
- Maintains an accurate screen for each session, including cursor, colors, and
  attributes.
- Waits for screen content instead of fixed delays.
- Sends text, named keys such as F1 or Tab, and key combinations.
- Records the host output that led to a failed wait, for diagnosis.
- Lets AI agents operate approved terminal applications through MCP.

TUICast works with existing applications. It does not require changes to the
application or the server it runs on.
