# Getting started with TUICast for TypeScript

TUICast lets Node.js applications control interactive terminal applications
over SSH or Telnet. The TypeScript SDK launches `tuicast-driver`, which owns the
network connection and terminal emulation.

## Prerequisites

- Node.js 24 or newer
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

Create a Node.js project and install the SDK from npm:

```sh
mkdir tuicast-typescript-demo
cd tuicast-typescript-demo
npm init -y
npm pkg set type=module
npm install @castingcode/tuicast@0.0.1
npm install --save-dev @types/node typescript
```

Node.js 24 can run erasable TypeScript directly, so this example does not need
an additional TypeScript runner.

## Choose credentials

The reference TUI has no default passwords, so choose your own and keep them
out of source code, just as you would for a real WMS environment. Store them in
a `.env.local` file that is excluded from version control:

```sh
cat > .env.local <<'EOF'
TUICAST_REFERENCE_PASSWORD=choose-an-ssh-password
TUICAST_REFERENCE_APP_PASSWORD=choose-an-application-password
EOF
echo .env.local >> .gitignore
```

Load the file into each terminal you use for this guide:

```sh
set -a; . ./.env.local; set +a
```

## Start the reference TUI

In a separate terminal, load `.env.local` and start TUICast's deterministic
demonstration server:

```sh
reference-tui --ssh-address 127.0.0.1:2222
```

Both the SSH transport and the application displayed inside the terminal use
the user ID `operator`. Their passwords are `TUICAST_REFERENCE_PASSWORD` and
`TUICAST_REFERENCE_APP_PASSWORD`, respectively.

## Automate a login

Create `quickstart.ts`:

```typescript
import { Driver, Keys } from "@castingcode/tuicast";

// Reads a password from the environment so that it never appears in source code.
function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

const driver = await Driver.launch();
let connection;
let session;

try {
  connection = await driver.connect({
    protocol: "ssh",
    address: "127.0.0.1:2222",
    username: "operator",
    password: requiredEnv("TUICAST_REFERENCE_PASSWORD"),
    // Safe only because this example connects to a local test server.
    insecureSkipHostKeyCheck: true,
  });
  session = await connection.openSession({ terminal: "xterm-256color" });

  await session.waitForText("LOGIN / AUTHENTICATION", { stableFor: 50 });
  await session.type("operator");
  await session.press(Keys.Tab);
  await session.type(requiredEnv("TUICAST_REFERENCE_APP_PASSWORD"));
  await session.press(Keys.Enter);

  const screen = await session.waitForText("TERMINAL TEST SYSTEM");
  if (!screen.contains("Authenticated as operator")) {
    throw new Error("login did not complete");
  }

  console.log("Login succeeded");
} finally {
  await session?.close();
  await connection?.close();
  await driver.close();
}
```

Type-check and run the example:

```sh
npx tsc --noEmit --module nodenext --target es2023 --types node quickstart.ts
node quickstart.ts
```

`waitForText` waits against the emulated terminal screen rather than raw
network output. The returned `Screen` is an immutable snapshot that can also
inspect lines, cells, colors, attributes, and the cursor. The `finally` block
closes every resource even when an operation fails.

For a real SSH server, replace `insecureSkipHostKeyCheck` with exactly one
trusted host-key option: `knownHostsFile` or `hostKeyFingerprint`. As in this
example, keep application and SSH credentials in environment variables or a
secret manager rather than in source code.
