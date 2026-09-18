<img src="tuicast-logo.svg" alt="tuicast" width="600">

# tuicast

## Table of Contents
- [Background](#Background)
- [Installation](#installation)

## Background
- Goal
  - To create a TUI automation framework, allowing you to control a terminal based application programatically. Use cases for such a framework include:
    - End-to-end (E2E) testing - simulate a real user flow and assert the app behaves correctly
    - Automation - expose a surface for terminal applications to be integrated into automated workflows

> TUICast makes terminal-based applications testable and automatable through reliable, programmatic control of interactive terminal sessions. It abstracts connection protocols and terminal behavior so engineers can interact with legacy TUIs using deterministic, modern automation APIs.

- Background and problem
  - Some enterprise systems still use interactive terminal interfaces over SSH or Telnet. Automating them can be challenging because they are stateful screen applications rather than command-line programs:
    - Output arrives incrementally.
    - Escape sequences update an existing screen.
    - Input includes special keys, not just text.
    - Applications often provide no explicit "ready" signal.
    - Tests may need hundreds of isolated, concurrent sessions.
  - TUICast provides one abstraction over those concerns.

- Intended users
  - Engineers testing legacy enterprise TUI applications
  - Teams integrating terminal-only systems into automated workflows
  - Test-tool and SDK authors
  - AI agents or orchestration systems that need controlled TUI access

- Primary use cases
  - End-to-end testing: Perform realistic user workflows and assert screen contents, cursor position, or other terminal state.
  - Business-process automation: Integrate terminal-only applications into larger workflows.
  - Concurrency and load scenarios: Run many independent users or sessions simultaneously.
  - Observability and diagnostics: Capture terminal state and failures in a form test tools can report.
  - Language-neutral integration: Control TUICast through its driver and language SDKs.
  - AI-assisted operation: Expose a constrained terminal automation surface through MCP  

- Glossary

## Installation

### SDK

The SDK launches tuicast-driver, so install that separately and make sure it is available on your PATH.

#### Go

To install the SDK for Go, use go get.
```
go get github.com/castingcode/tuicast/sdk/go@latest
```
Then in your code, import the SDK as:
```
import tuicast "github.com/castingcode/tuicast/sdk/go"
```

### Driver and Other Binaries - Using the install script

The default destination is ./bin. Add the selected directory to PATH if necessary.

#### macOS and Linux

Install the driver into ./bin:
```
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh | sh
```

Install the reference TUI:
```
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component reference-tui
```

Install everything into ~/.local/bin:
```
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component all --bin-dir "$HOME/.local/bin"
```

Install a specific version:
```
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component driver v0.0.1
```

#### Windows PowerShell
Install the driver:
```
$install = [scriptblock]::Create((irm `
  https://github.com/castingcode/tuicast/releases/latest/download/install.ps1))

& $install -Component driver
```

Reference TUI:
```
& $install -Component reference-tui
```

Everything in a chosen directory:
```
& $install -Component all -BinDir "$HOME\bin"
```

Specific version:
```
& $install -Component driver -Version v0.0.1
```

### Driver and Other Binaries - Using Go install

Alternatively, you can use `go install` to install the binaries.
Ensure $GOBIN or $GOPATH/bin is on PATH, as the SDK will launch the driver.

To install, run:
```
go install github.com/castingcode/tuicast/cmd/tuicast-driver@latest
```

The reference TUI and MCP server also support go install:
```
go install github.com/castingcode/tuicast/cmd/reference-tui@latest
go install github.com/castingcode/tuicast/cmd/tuicast-mcp@latest
```
