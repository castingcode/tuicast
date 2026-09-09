Are there any good reference TUI applications that can be used for testing and demos?  Ideally they would be something that can be run locally in a container and is open source 



Yes. There are several good options, but **I would actually use a small collection of reference applications rather than trying to find one perfect application**. Your server needs to exercise different aspects of terminal behavior: screen updates, cursor movement, colors, alternate screens, function keys, resizing, input handling, and long-running sessions.

### My top choices

| Application                                                                              | Why it's useful                                         | Docker/local               | Best for                                 |
| ---------------------------------------------------------------------------------------- | ------------------------------------------------------- | -------------------------- | ---------------------------------------- |
| **[Lodestar / LBBS](https://github.com/InterLinked1/lbbs?utm_source=chatgpt.com)**       | Full BBS server supporting Telnet, SSH, RLogin and ANSI | Yes                        | **SSH/Telnet integration testing**       |
| **[SyncTERM Web Service](https://github.com/jaromaz/stws?utm_source=chatgpt.com)**       | Dockerized SyncTERM/BBS environment                     | Yes                        | Telnet/ANSI testing                      |
| **[terminaltui](https://terminaltui.dev/?utm_source=chatgpt.com)**                       | Modern TUI apps specifically designed to run over SSH   | Yes-ish / easy local setup | **Automated TUI testing**                |
| **[TamboUI demos](https://github.com/tamboui/tamboui?utm_source=chatgpt.com)**           | Large collection of purpose-built TUI demos             | Buildable locally          | Terminal rendering/emulation             |
| **[Helapordo](https://github.com/jgabaut/helapordo?utm_source=chatgpt.com)**             | ncurses-based roguelike                                 | Buildable                  | ncurses behavior                         |
| **[Sentry WMS](https://github.com/hightower-systems/sentry-wms?utm_source=chatgpt.com)** | Actual open-source WMS                                  | Docker Compose             | WMS-domain reference, though **not TUI** |

Of these, **LBBS is probably the most interesting starting point for your particular project**. It supports both Telnet and SSH and has ANSI terminal behavior, so you can run something resembling the environment your server will eventually connect to. ([GitHub][1])

### But I'd build a purpose-built "TUI Test Application"

I think this is actually the most valuable approach for your project.

Rather than relying exclusively on third-party applications, I'd create a small **reference TUI application specifically designed to test your terminal server**.

For example:

```text
┌──────────────────────────────────────────────────────────────┐
│                    TERMINAL TEST SYSTEM                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Login / Authentication                                   │
│  2. Forms                                                    │
│  3. Tables                                                   │
│  4. Scrolling                                                │
│  5. Colors                                                   │
│  6. Cursor Movement                                          │
│  7. Function Keys                                            │
│  8. Input Fields                                             │
│  9. Long Running Operation                                   │
│ 10. Terminal Resize                                          │
│ 11. Unicode                                                  │
│ 12. ANSI / VT220 Tests                                       │
│                                                              │
│                    Select: _                                  │
└──────────────────────────────────────────────────────────────┘
```

Then deliberately include screens that exercise difficult terminal behavior:

```text
LOGIN

User ID:  ______________________
Password: ______________________

[ F1 ] Login       [ F2 ] Cancel
```

Then:

```text
ORDERS

Order       Customer       Status       Qty
──────────────────────────────────────────────
10002341    ACME           PICKING       42
10002342    Acme Corp      SHIPPED       18
10002343    Widget Inc     ALLOCATED     73
...
```

And importantly, have operations that cause:

* partial screen updates
* scrolling
* cursor repositioning
* protected fields
* function-key handling
* arrow keys
* Enter/Escape
* Ctrl-key combinations
* terminal resize
* color changes
* alternate screen buffer
* blinking/reverse video
* Unicode
* long-running output
* asynchronous screen changes
* intentionally delayed responses

That would give you a **deterministic integration-test target** for the server.

### This could become especially valuable for your project

I'd consider having a repository structure along these lines:

```text
castingcode/
├── terminal-server/
│
├── terminal-test-app/
│
└── terminal-test-suite/
```

Or, if you want everything in one repository:

```text
cmd/
    terminal-server/
    test-tui/

internal/
    session/
    terminal/
    ssh/
    telnet/

test/
    integration/
```

The `test-tui` could expose a deliberately WMS-like workflow:

```text
LOGIN
  ↓
MAIN MENU
  ↓
1. Warehouse Operations
  ↓
2. Receiving
  ↓
3. Receive Purchase Order
  ↓
PO NUMBER: 123456
  ↓
ITEM: ABC-123
  ↓
QUANTITY: 25
  ↓
CONFIRM
```

That would let your automated tests read almost like real WMS automation:

```go
session := server.NewSession(...)

session.ExpectText("MAIN MENU")
session.Send("1")
session.ExpectText("WAREHOUSE OPERATIONS")

session.Send("2")
session.ExpectText("RECEIVING")

session.Send("3")
session.ExpectText("PURCHASE ORDER")

session.Type("123456")
session.Press(Enter)

session.ExpectText("ITEM:")
```

**That is much closer to the Playwright mental model you're aiming for** than simply testing against a generic TUI application.

### One particularly interesting existing project

`terminaltui` caught my attention because it explicitly supports hosting TUI applications over SSH and has multiple reference applications. Its documentation even shows independent SSH sessions with different `TERM` values such as `xterm-256color` and `vt100`. ([terminaltui][2])

That makes it potentially useful for **protocol/emulation compatibility testing**, while your own purpose-built TUI can become the deterministic functional-test target.

I'd therefore use **three layers of test targets**:

1. **Your own `test-tui`** — deterministic functional/integration tests.
2. **terminaltui/TamboUI/etc.** — modern TUI rendering and terminal-emulation edge cases.
3. **LBBS/BBS applications** — real SSH/Telnet server behavior and long-lived concurrent sessions.

That combination would give you a much stronger test environment than trying to find a single "reference WMS TUI."

[1]: https://github.com/InterLinked1/lbbs?utm_source=chatgpt.com "GitHub - InterLinked1/lbbs: Lightweight BBS For Linux - Bulletin Board System server software · GitHub"
[2]: https://terminaltui.dev/?utm_source=chatgpt.com "terminaltui — Interactive terminal websites and apps in TypeScript"
