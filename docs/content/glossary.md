---
title: "Glossary"
description: "Definitions of terminal, protocol, and TUICast terms, from VT220, Telnet, and escape sequence to session, matcher, stable wait, and flight recorder."
weight: 50
---

Terms are listed alphabetically. Terms specific to TUICast are marked
*(TUICast)*.

**Alternate screen**
: A second screen buffer that full-screen applications switch to while
  running, so the original screen returns when they exit. Supported by the
  `xterm-256color` profile.

**Answerback**
: The text a terminal sends back when the host sends ENQ. Some applications
  use it to identify a device. TUICast sends the session's configured
  answerback, up to 20 characters.

**BELL**
: A control character (`0x07`) that makes a terminal beep. Warehouse and RF
  applications often beep on errors. TUICast reports each bell as an event.

**Block-mode terminal**
: A terminal, such as the IBM 3270 or 5250, that sends a whole screen of
  fields at once instead of each keystroke. TUICast does not support
  block-mode terminals. Compare *character-mode terminal*.

**Cell**
: One character position on the screen, with its character, colors, and
  attributes.

**Character-mode terminal**
: A terminal that sends each keystroke to the host as it is typed and
  displays whatever the host sends back. VT100, VT220, and xterm are
  character-mode terminals.

**Connection** *(TUICast)*
: One SSH or Telnet network link to a host. A connection carries one or more
  sessions.

**Control sequence**
: An escape sequence that starts with `ESC [` (written `CSI`), such as
  `CSI 2 J`, which clears the screen.

**DEC Special Graphics**
: A VT100 and VT220 character set used to draw lines and boxes on screen.

**Driver** *(TUICast)*
: The `tuicast-driver` program. It manages connections, emulates terminals,
  and performs waits on behalf of an SDK.

**ENQ**
: A control character (`0x05`, "enquiry") that asks a terminal to send its
  answerback.

**Escape sequence**
: A series of bytes beginning with the Escape character (`0x1B`) that tells a
  terminal to do something other than display text, such as move the cursor
  or change color.

**Flight recorder** *(TUICast)*
: Each session's record of its most recent host output, up to 64 KiB. A
  failed wait includes a copy, called a *dump*, for diagnosis.

**Green screen**
: An informal name for text-based enterprise applications, after the
  monochrome terminals they once ran on. It is used for both character-mode
  (VT) and block-mode (3270 and 5250) applications. TUICast supports the
  character-mode kind.

**Host**
: The server that runs the terminal application TUICast connects to.

**Host key**
: The public key an SSH server uses to prove its identity. TUICast verifies
  it against a `known_hosts` file or a fingerprint.

**Idle wait** *(TUICast)*
: A wait that succeeds when the host has sent nothing for a *quiet period*,
  without checking the screen's content.

**Inspector** *(TUICast)*
: An optional, read-only web page served by the driver that shows live
  connections, sessions, and screens.

**JSON-RPC**
: A simple protocol for calling methods by sending JSON messages. SDKs use it
  to control the driver.

**Matcher** *(TUICast)*
: A description of what a screen must show for a wait to succeed, such as
  "contains `ORDER SHIPPED`".

**MCP (Model Context Protocol)**
: An open protocol that lets AI applications use external tools. The TUICast
  MCP server lets AI agents operate terminal applications.

**NAWS**
: "Negotiate About Window Size", the Telnet option a terminal uses to tell
  the host its screen size.

**Profile (MCP)** *(TUICast)*
: A named, operator-approved connection in the MCP server's configuration.
  Not to be confused with a *terminal profile*.

**Quiet period** *(TUICast)*
: The length of time the host must send nothing before a stable or idle wait
  succeeds.

**Reference TUI** *(TUICast)*
: A practice terminal application included with TUICast, used for tutorials,
  demos, and testing TUICast itself.

**Revision** *(TUICast)*
: A number that increases each time a session's visible screen changes.

**RF terminal**
: A handheld or vehicle-mounted radio-frequency device, often called an RF
  gun, that warehouse workers use to scan barcodes and run terminal
  applications, usually over Telnet or SSH with VT220 emulation.

**Screen snapshot** *(TUICast)*
: An unchangeable copy of a session's screen at one moment.

**SDK** *(TUICast)*
: A library for Go, Python, TypeScript, or Java that controls the driver.

**Session** *(TUICast)*
: One interactive terminal on a connection: one screen, one cursor, one
  simulated user.

**SSH**
: Secure Shell, an encrypted protocol for signing in to and using a remote
  computer.

**SSH channel**
: One stream within an SSH connection. Each TUICast session over SSH uses its
  own channel.

**Stable wait** *(TUICast)*
: A wait that requires its matcher to succeed and the host to then be silent
  for a quiet period.

**Telnet**
: An older, unencrypted protocol for using a remote computer through a
  terminal. Still common in warehouses for RF terminals.

**Telnet option negotiation**
: The exchange at the start of a Telnet connection in which each side agrees
  on features such as terminal type and window size. TUICast handles it
  automatically.

**TERM**
: The name of the terminal type a host believes it is talking to, such as
  `vt220`. TUICast reports the session's terminal profile name.

**Terminal emulator**
: Software that imitates a physical terminal. TUICast contains one for each
  terminal profile.

**Terminal profile** *(TUICast)*
: The kind of terminal a session imitates: `vt220` or `xterm-256color`.

**TUI (text user interface)**
: An application that draws screens, menus, and forms with text characters in
  a terminal.

**Unsupported sequence** *(TUICast)*
: An escape sequence a terminal profile does not implement. TUICast ignores it
  and reports it as an event.

**VT100, VT220**
: Digital Equipment Corporation (DEC) terminals from 1978 and 1983 whose
  behavior became the standard that most terminal emulators follow.

**Wait** *(TUICast)*
: An operation that pauses until the screen matches a matcher, or fails after
  a timeout.

**WMS (warehouse management system)**
: Software that runs warehouse operations such as receiving, picking, and
  shipping. Many WMS products offer terminal interfaces for RF terminals.

**Workbench** *(TUICast)*
: An optional browser interface, built on the Inspector, for recording
  workflows from a live session.

**xterm**
: The standard terminal emulator for the X Window System. Its behavior, a
  superset of the VT220's, is what most modern terminal applications expect.
