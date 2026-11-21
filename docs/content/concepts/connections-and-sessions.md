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

Sessions run independently of each other, allowing multiple simulations to run at once. Each session has its own screen, cursor, terminal type and size. 

## How Sessions End

A session ends when:

- the connection fails
- the code closes it
- the application closes it
    -In the scenario that the application ends a session, the final screen remains and cannot be edited until the user closes the session.  


## Closing Releases Everything Below

Closing works top-down:

- closing the driver closes every connection
- closing a connection closes its sessions
- closing a session releases its terminal

Always close the driver when your program finishes, even after a failure. Each
SDK provides an idiomatic way to do this, such as `defer` in Go or `with` in
Python.
