---
title: "Connect to your terminal application"
linkTitle: "Connect to your application"
description: "Connect TUICast to your own terminal application over SSH or Telnet: verify host keys, use passwords or private keys, and match the terminal type and size."
weight: 10
---

To move from the reference TUI to your own application, you need four facts:
how users connect (SSH or Telnet), how they sign in, which terminal type their
emulator or device uses, and its screen size. This guide uses Go; every SDK
uses the same option names in its own style, for example `known_hosts_file` in
Python.

## Connect over SSH

Give TUICast the host, a user name, and one way to sign in: a password, a
private key, or both.

```go
connection, err := driver.Connect(ctx, tuicast.SSH{
	Address:        "wms-test.example.test:22",
	Username:       os.Getenv("WMS_SSH_USER"),
	PrivateKey:     os.Getenv("WMS_SSH_KEY"),
	KnownHostsFile: "/home/ci/.ssh/known_hosts",
	Timeout:        30 * time.Second,
})
```

| Option | Purpose |
| --- | --- |
| `Password` | Password sign-in |
| `PrivateKey` | PEM-encoded private key contents |
| `PrivateKeyPassphrase` | Passphrase for an encrypted private key |
| `KnownHostsFile` | Verify the host against a `known_hosts` file on the driver's machine |
| `HostKeyFingerprint` | Verify the host against its SHA-256 fingerprint |
| `InsecureSkipHostKeyCheck` | Skip verification; local test servers only |
| `Timeout` | Maximum time to establish the connection |

Set exactly one host-key option. A fingerprint is written as OpenSSH prints
it, for example `SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s`. To get
it, ask the server's administrator or run
`ssh-keyscan HOST | ssh-keygen -lf -` from a trusted network.

## Connect over Telnet

Telnet needs only an address. Signing in happens on screen, like a user would:

```go
connection, err := driver.Connect(ctx, tuicast.Telnet{
	Address: "legacy-test.example.test:23",
})
```

Then wait for the host's login prompt and type the credentials, as in the
getting-started guides. Telnet is unencrypted; use it only on networks you
trust.

## Match the terminal

Open the session with the terminal type and size your users have:

```go
session, err := connection.OpenSession(ctx,
	tuicast.WithTerminal(tuicast.VT220),
	tuicast.WithSize(40, 16),
)
```

If screens look different from what users see, check the terminal type and
size first. See [Terminal emulation](../concepts/terminal-emulation.md).

If the application sends ENQ and expects a particular answerback, such as a
device identifier, set it with `WithAnswerback`.

## Check the first screen

Before writing a full test, confirm the connection works:

1. Wait for text from the first screen, such as the application's sign-in
   prompt.
2. Print the screen's text and compare it with what users see.
3. Check for unsupported-sequence events. A few are normal; a screen that
   looks wrong usually comes with some.

If the first wait times out, see
[Diagnose failed waits](diagnose-failed-waits.md).
