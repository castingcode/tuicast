---
title: "Frequently asked questions"
linkTitle: "FAQ"
description: "Answers to common questions about TUICast: supported terminals and protocols, languages, licensing, IBM 3270 and 5250, scale, AI agents, and failed waits."
weight: 60
---

### What is TUICast?

TUICast is an open-source tool for testing and automating interactive
terminal applications over SSH and Telnet. It emulates VT220 and xterm
terminals and provides SDKs for Go, Python, TypeScript, and Java.

### Do I need to change my application?

No. TUICast connects the same way a user's terminal does and sees only what
the terminal would see.

### Which terminals and protocols does TUICast support?

TUICast connects over SSH or Telnet and emulates `vt220` and `xterm-256color`
terminals. See [Terminal support](reference/terminal-support.md).

### Can TUICast automate IBM 3270 or 5250 (AS/400) applications?

No. Those are block-mode terminals with different protocols (TN3270 and
TN5250). TUICast supports character-mode terminals such as the VT220 and
xterm.

### Does TUICast work with RF warehouse applications?

Yes, when the application is used over SSH or Telnet with a VT220 or xterm
terminal type, which is common for RF terminals. Set the session to the same
terminal type and screen size as the devices.

### Which programming languages can I use?

Go, Python, TypeScript, and Java have SDKs. Any other language can control the
driver through its [JSON-RPC protocol](reference/driver-protocol.md).

### How is TUICast different from expect or pexpect?

expect and pexpect match the raw text an application sends. TUICast applies
that output to an emulated terminal and matches the resulting screen, which
is what full-screen applications are designed around.

### How many sessions can I run at once?

TUICast is designed for many concurrent sessions. The practical limits are
usually on the host: SSH servers limit sessions per connection (10 by default
in OpenSSH), and each simulated user may need its own application account.
See [Connections and sessions](concepts/connections-and-sessions.md).

### Is it safe to give an AI agent access?

The MCP server restricts agents to connections an operator approves, and
agents never see addresses or credentials. Review the
[security model](concepts/security-model.md#ai-agents) against your
organization's AI policy.

### Why does my wait time out?

Usually the application showed something different, the matcher is too
strict, or the screen was still being drawn. See
[Diagnose failed waits](guides/diagnose-failed-waits.md).

### Which operating systems are supported?

The driver, MCP server, and reference TUI run on macOS, Linux, and Windows, on
amd64 and arm64 processors. Container images are available for Linux on amd64
and arm64.

### What license does TUICast use?

TUICast and its SDKs are licensed under the MIT License.

### Why is it called TUICast?

TUICast is developed by CastingCode. The name joins *TUI* (text user interface)
with *cast*, from fly fishing.
<!-- WRITER: confirm the naming story with the maintainer, or remove this question. -->
