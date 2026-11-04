---
title: "Run many terminal sessions concurrently"
linkTitle: "Run sessions concurrently"
description: "Run many simultaneous TUICast terminal sessions for parallel tests or load scenarios, within SSH channel limits and one driver per test worker."
weight: 30
draft: true
---

## Writer brief

**Reader question:** "How do I simulate 50 RF users at once, or run my test
suite in parallel?"

**Cover:**

- One driver process per test worker; why this isolates workers (link
  [How TUICast works](../concepts/how-tuicast-works.md)).
- Many sessions in one driver: sessions are independent and waits run
  concurrently.
- SSH limits: sessions share a connection as channels; OpenSSH `MaxSessions`
  defaults to 10, so spread sessions across several connections. Telnet needs
  one connection per session.
- Practical limits to watch on the host side: license seats, user accounts,
  and server connection limits. Each simulated user usually needs its own
  application account.
- A short example in one language, taken from the concurrent-sessions example
  in `examples/` (each language has one).

**Sources:** `design/session-lifecycle.md` (Transport Topology),
`examples/*/reference/` concurrent-session examples.

**Length:** about 400 words.
