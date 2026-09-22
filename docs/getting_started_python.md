# Getting started with TUICast for Python

TUICast lets Python programs control interactive terminal applications over
SSH or Telnet. The Python SDK launches `tuicast-driver`, which owns the network
connection and terminal emulation.

## Prerequisites

- Python 3.12 or newer
- `tuicast-driver` installed and available on `PATH`

On macOS or Linux, install the driver and the reference TUI used by this guide:

```sh
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component all --bin-dir "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
```

See the [main installation instructions](../README.md#driver-and-other-binaries---using-the-install-script)
for Windows and other installation options.

## Install the SDK

Create a virtual environment for the application and install TUICast from
PyPI:

```sh
mkdir tuicast-python-demo
cd tuicast-python-demo
python3.12 -m venv .venv
source .venv/bin/activate
python -m pip install tuicast==0.0.1
```

## Start the reference TUI

In a separate terminal, start TUICast's deterministic demonstration server:

```sh
reference-tui \
  --ssh-address 127.0.0.1:2222 \
  --ssh-username demo \
  --ssh-password demo-password
```

The SSH transport uses `demo` / `demo-password`. The application displayed
inside the terminal has its own test login: `operator` / `casting`.

## Automate a login

Create `quickstart.py`:

```python
import tuicast


with tuicast.Driver.launch() as driver:
    with driver.connect(
        tuicast.SSH(
            "127.0.0.1:2222",
            "demo",
            password="demo-password",
            # Safe only because this example connects to a local test server.
            insecure_skip_host_key_check=True,
        )
    ) as connection:
        with connection.open_session(terminal=tuicast.Terminal.XTERM_256COLOR) as session:
            session.wait_for_text("LOGIN / AUTHENTICATION", stable_for=0.05)
            session.type("operator")
            session.press(tuicast.Key.TAB)
            session.type("casting")
            session.press(tuicast.Key.ENTER)

            screen = session.wait_for_text("TERMINAL TEST SYSTEM")
            if not screen.contains("Authenticated as operator"):
                raise RuntimeError("login did not complete")

            print("Login succeeded")
```

Run it from the activated virtual environment:

```sh
python quickstart.py
```

`wait_for_text` waits against the emulated terminal screen rather than raw
network output. The returned `Screen` is an immutable snapshot that can also
inspect lines, cells, colors, attributes, and the cursor. The context managers
close the session, connection, and driver even when an operation fails.

For a real SSH server, replace `insecure_skip_host_key_check=True` with exactly
one trusted host-key option: `known_hosts_file` or `host_key_fingerprint`. Keep
application and SSH credentials in environment variables or a secret manager.
