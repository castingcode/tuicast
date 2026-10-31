---
title: "Connections and sessions"
description: "How TUICast connections and terminal sessions relate, how SSH and Telnet differ, and when sessions end and resources are released."
weight: 20
---

In TUICast, a **connection** is one network link to a host, and a **session**
is one interactive terminal on that connection, equivalent to one user at one
terminal. You open a connection, open one or more sessions on it, and interact
with each session independently.

```text
Driver
└── Connection  (one SSH or Telnet link)
    └── Session (one terminal: screen, cursor, input)
```

## SSH and Telnet Differ

| | SSH | Telnet |
| --- | --- | --- |
| Sessions per connection | Several, as separate SSH channels | One |
| Authentication | Once per connection, by TUICast | On screen, by your code typing at the host's login prompt |
| Encryption | Yes | No |

Several SSH sessions can share one connection and its sign-in. SSH servers
usually limit how many; OpenSSH allows 10 by default (`MaxSessions`). To run
more sessions, open more connections. See
[Run sessions concurrently](../guides/run-sessions-concurrently.md).

A Telnet connection carries exactly one session, so each simulated user needs
its own connection.

## Each Session is Isolated

Each session has its own screen, cursor, terminal type, and size. Typing into
one session never affects another, and waits on different sessions run
independently. This is what lets one test drive many simulated users at once.

## How Sessions End

A session ends when:

- the application closes it, for example after the user signs out;
- the connection fails; or
- your code closes it.

When the application ends a session, TUICast treats that as a normal outcome,
not an error. The final screen stays readable until you close the session, so a
test can check a sign-out message. Sending input to an ended session fails.
Your code can wait for a session to end, which is useful for testing sign-out.

## Closing Releases Everything Below

Closing is safe to repeat and works top-down:

- closing a session releases its terminal;
- closing a connection closes its sessions;
- closing the driver closes every connection.

Always close the driver when your program finishes, even after a failure. Each
SDK provides an idiomatic way to do this, such as `defer` in Go or `with` in
Python.
