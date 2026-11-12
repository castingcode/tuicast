---
title: "Connections and sessions"
description: "How TUICast connections and terminal sessions relate, how SSH and Telnet differ, and when sessions end and resources are released."
weight: 20
---

A connection is a single network link to a host. Sessions are interactive terminals on a connection that equate to one user per terminal. TUICast users can open a connection with multiple sessions and interact with each independently.

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

Multiple SSH sessions can share a single connection and sign-in, opening more connections creates more sessions ([Run sessions concurrently](https://github.com/castingcode/tuicast/blob/draft-docs/docs/content/guides/run-sessions-concurrently.md)). OpenSSH allows up to 10 by default, but this can be changed if desired (sshd_config -> MaxSessions). 

Telnet differs as only a single session can be in a connection, one session per connection.

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
