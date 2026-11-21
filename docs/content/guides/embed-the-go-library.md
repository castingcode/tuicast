---
title: "Embed the TUICast Go library"
linkTitle: "Embed the Go library"
description: "Use TUICast as a Go library inside your own program, without a separate driver process, by composing SSH or Telnet transports with a terminal profile."
weight: 80
draft: true
---

## Writer brief

**Reader question:** "I am writing a Go service. Can I use TUICast directly
instead of launching a driver?"

**Cover:**

- When to embed (Go service, custom tooling, in-memory tests) and when to use
  the Go SDK instead (test suites; keeps your dependencies small).
- The pieces you compose: `tuicast.NewServer`, a connector from the `ssh` or
  `telnet` package, and a terminal from `vt220` or `xterm`.
- The `memory` package for tests that need no network.
- A complete, compiling example. Ask an engineer to provide and test it.

**Sources:** `design/architecture.md`, `design/session-lifecycle.md`,
`cmd/tuicast-driver/main.go` (shows the composition), package documentation
on pkg.go.dev.

**Length:** about 400 words plus the example.
