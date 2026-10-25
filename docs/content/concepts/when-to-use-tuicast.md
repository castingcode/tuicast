---
title: "When to use TUICast"
description: "How TUICast compares with expect, pexpect, terminal screen-scraping libraries, and RPA tools for testing and automating interactive terminal applications."
weight: 60
draft: true
---

## Writer brief

**Reader question:** "I already know expect, pexpect, or our RPA tool. Why
would I use TUICast, and when should I not?"

**Why it matters:** Comparison questions are among the most common searches
and AI prompts during tool evaluation. A fair, factual page is often the one
an AI engine quotes.

**Cover:**

- The core difference: stream-based tools (expect, pexpect, and plain SSH
  libraries) match bytes as they arrive, while TUICast matches an emulated
  screen. Explain why that matters for full-screen applications that redraw
  in place.
- A comparison table with columns for TUICast, expect/pexpect, a general SSH
  library, and RPA terminal connectors (for example UiPath or Blue Prism).
  Rows: screen model, SSH/Telnet, VT220/xterm emulation, waits, concurrency,
  languages, AI/MCP support, license and cost.
- Where TUICast is not the right tool: IBM 3270/5250 block-mode applications
  (suggest s3270 or x3270 scripting), simple line-oriented commands (a plain
  SSH library is enough), and GUI applications.
- One sentence on the Playwright analogy and where it holds.

**Sources:** [How TUICast works](how-tuicast-works.md),
[Screens and waiting](screens-and-waiting.md), each tool's official
documentation. Cite sources for every claim about another tool and keep the
tone neutral.

**Out of scope:** benchmarks and feature promises.

**Length:** about 600 words, mostly the table.
