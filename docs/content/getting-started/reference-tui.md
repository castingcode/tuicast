---
title: "Run the reference TUI"
linkTitle: "Reference TUI"
description: "Run the TUICast reference TUI, a practice terminal application with forms, tables, and function keys, locally or over SSH and Telnet."
weight: 20
---

The reference TUI is a practice terminal application that ships with TUICast.
It behaves like a small warehouse system, with a login screen, forms, tables,
function keys, colors, and slow operations, and it always behaves the same way,
which makes it suitable for tutorials, demos, and tests. Use it to learn
TUICast before connecting to your own application.

## Set passwords

The reference TUI has no default passwords, just as a real system should not.
Choose two and export them:

```sh
export TUICAST_REFERENCE_PASSWORD='choose-an-ssh-password'
export TUICAST_REFERENCE_APP_PASSWORD='choose-an-app-password'
```

| Variable | Used for |
| --- | --- |
| `TUICAST_REFERENCE_PASSWORD` | Signing in over SSH |
| `TUICAST_REFERENCE_APP_PASSWORD` | The application's login screen (at most 24 characters) |

Both user IDs are `operator`. For convenience, the login screen displays the
application credentials.

In a clone of the repository, copy `.env.example` to `.env.local`, which Git
ignores, set the passwords there, and load both files:

```sh
set -a; . ./.env; . ./.env.local; set +a
```

## Run it

Run it in your terminal to see what you will automate:

```sh
reference-tui
```

Serve it over SSH, which the getting-started guides use:

```sh
reference-tui -ssh-address 127.0.0.1:2222
```

Or serve it over Telnet:

```sh
reference-tui -telnet-address 127.0.0.1:2323
```

One process serves one protocol. Start two processes to serve both. Press
`Ctrl-C` to stop.

To connect by hand from another terminal:

```sh
ssh -tt -o PreferredAuthentications=password -o PubkeyAuthentication=no \
  operator@127.0.0.1 -p 2222
```

For every option, including additional users and a fixed SSH host key, see the
[command-line reference](../reference/command-line.md#reference-tui).

## Optional: VTTEST

The reference TUI can launch the external `vttest` terminal test program from
its menu when `vttest` is on your `PATH`. Install it with `brew install vttest`
or your Linux package manager.

The reference TUI's passwords and host keys are for local testing only. Do not
expose it on a shared network.
