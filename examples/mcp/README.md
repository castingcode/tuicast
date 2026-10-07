# TUICast MCP examples

These examples connect an MCP-capable coding agent to TUICast's deterministic
reference terminal. The MCP server exposes terminal automation tools while an
operator-controlled profile fixes the endpoint, credentials source, terminal
type, and dimensions.

## Prerequisites

- Go 1.24 or newer when building from this checkout
- VS Code with GitHub Copilot, Cursor, or another MCP client with stdio support
- `tuicast-mcp` and `reference-tui` available on `PATH`

Build and install the two commands from the repository root:

```sh
go install ./cmd/tuicast-mcp ./cmd/reference-tui
```

Ensure Go's binary directory is on the environment inherited by the editor:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Start the reference terminal

In a separate terminal, start the local SSH fixture:

```sh
reference-tui --ssh-address 127.0.0.1:2222
```

The fixture uses `operator` / `casting` for both SSH authentication and the
application login. The example profile disables SSH host-key checking only for
this local deterministic server. Use a known-hosts file or pinned fingerprint
for any real endpoint.

## Configure an MCP client

The checked-in files under [`clients`](clients) are templates. If the target
configuration already exists, merge the `tuicast-reference` entry into it
instead of overwriting the file.

### VS Code

Copy [`clients/vscode.json`](clients/vscode.json) to `.vscode/mcp.json` and
start the `tuicast-reference` server from VS Code's MCP controls.

VS Code launched from the Dock or Finder does not inherit your shell's `PATH`.
If the server log reports `spawn tuicast-mcp ENOENT`, set `command` to the
absolute path printed by `command -v tuicast-mcp`, such as
`/Users/you/go/bin/tuicast-mcp`.

The template sets the fixture's published password directly because VS Code
does not forward servers that use `${input:...}` prompts to Copilot CLI (Agent
Host) chat sessions. For a real endpoint, keep credentials out of the
workspace: use `envFile` with an untracked file or inherit the variable from
the environment.

### Cursor

Export the reference password before launching Cursor so its MCP subprocess can
read it:

```sh
export TUICAST_REFERENCE_PASSWORD=casting
cursor .
```

Copy [`clients/cursor.json`](clients/cursor.json) to `.cursor/mcp.json`, then
enable `tuicast-reference` under **Cursor Settings → Tools & MCP**. The template
uses Cursor's project-level `mcpServers` format and environment interpolation.

## Run a workflow

Paste one of these prompts into the editor's agent mode:

- [`prompts/login.md`](prompts/login.md) — discover the approved profile, log
  in, verify the main menu, and clean up.
- [`prompts/receiving-form.md`](prompts/receiving-form.md) — navigate and submit
  a realistic warehouse receiving form.
- [`prompts/function-keys.md`](prompts/function-keys.md) — send a named function
  key and a modified character.
- [`prompts/write-test.md`](prompts/write-test.md) — explore a workflow, record
  a clean run, and turn the recording into a test using one of the TUICast
  SDKs. Run it from a checkout or workspace that contains the SDK's getting
  started guide.

Approve the TUICast tool calls when requested. The prompts require explicit
session and connection cleanup. As a final safeguard, `tuicast-mcp` also closes
its complete object graph when the MCP client disconnects.

## What the example demonstrates

The agent can list approved profile metadata, connect, open a session, inspect
the current screen, wait for text, matcher expressions, or idle output, type,
press keys, record a replayable workflow, and close resources.
It cannot supply an arbitrary hostname, port, username, credential, terminal
profile, or screen size. Those values remain in
[`reference-profile.json`](reference-profile.json), outside the model's tool
arguments, and credentials are loaded from the named environment variable.

See [`design/mcp.md`](../../design/mcp.md) for the full tool and security
contract.
