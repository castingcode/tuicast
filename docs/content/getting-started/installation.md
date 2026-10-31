---
title: "Install TUICast"
linkTitle: "Installation"
description: "Install the TUICast driver, MCP server, and reference TUI with Homebrew, WinGet, Scoop, npm, Linux packages, an install script, or go install, then add an SDK."
weight: 10
---

TUICast has two parts to install: the **driver** binary, which every SDK
launches, and an **SDK** for your programming language. The driver must be on
your `PATH`.

| Component | Binary | Needed for |
| --- | --- | --- |
| Driver | `tuicast-driver` | Every SDK |
| Reference TUI | `reference-tui` | Tutorials, demos, and trying TUICast |
| MCP server | `tuicast-mcp` | AI agents only |

## Install the binaries

Choose one method. Homebrew, WinGet, Scoop, and the Linux packages install all
three binaries.

### macOS: Homebrew

```sh
brew install --cask castingcode/tap/tuicast
```

### Windows: WinGet or Scoop

```powershell
winget install --exact --id CastingCode.TUICast
```

```powershell
scoop bucket add castingcode https://github.com/castingcode/scoop-bucket.git
scoop install castingcode/tuicast
```

### Linux: deb, rpm, or apk

Download the package for your CPU from the
[latest release](https://github.com/castingcode/tuicast/releases/latest) and
install it with your package manager, for example:

```sh
sudo apt install ./tuicast_VERSION_linux_amd64.deb
```

### macOS and Linux: install script

The script installs into `./bin` by default. This command installs every
binary into `~/.local/bin`:

```sh
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component all --bin-dir "$HOME/.local/bin"
```

Use `--component` with `driver` (the default), `mcp`, `reference-tui`, or
`all`. Add a version, such as `v0.2.1`, as the last argument to pin a release.

### Windows: install script

```powershell
$install = [scriptblock]::Create((irm `
  https://github.com/castingcode/tuicast/releases/latest/download/install.ps1))
& $install -Component all -BinDir "$HOME\bin"
```

Use `-Version v0.2.1` to pin a release.

### npm

```sh
npm install --global @castingcode/tuicast-cli
```

This installs `tuicast-driver` and `reference-tui` for your operating system
and verifies their SHA-256 checksums. It is separate from the
`@castingcode/tuicast` TypeScript SDK.

### Go

```sh
go install github.com/castingcode/tuicast/cmd/tuicast-driver@latest
go install github.com/castingcode/tuicast/cmd/reference-tui@latest
go install github.com/castingcode/tuicast/cmd/tuicast-mcp@latest
```

Make sure `$GOBIN` or `$GOPATH/bin` is on your `PATH`.

### Containers

Linux images for amd64 and arm64 are published on Docker Hub as
`castingcode/tuicast-driver` and `castingcode/tuicast-reference-tui`. See
[Run in containers and CI](../guides/run-in-containers-and-ci.md).

## Check the installation

```sh
tuicast-driver -version
```

The command prints the version, commit, and build date. If the shell cannot
find `tuicast-driver`, add the install directory to your `PATH`.

## Install an SDK

| Language | Requirement | Install |
| --- | --- | --- |
| Go | Go 1.27.3+ | `go get github.com/castingcode/tuicast/sdk/go@latest` |
| Python | Python 3.12+ | `python -m pip install tuicast` |
| TypeScript | Node.js 24+ | `npm install @castingcode/tuicast` |
| Java | Java 21+ | Maven `dev.castingcode:tuicast` |

Next, run the [reference TUI](reference-tui.md) or go straight to your
language's guide.
