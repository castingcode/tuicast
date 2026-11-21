---
title: "Run TUICast in containers and CI pipelines"
linkTitle: "Run in containers and CI"
description: "Run TUICast tests in GitHub Actions, Azure Pipelines, or other CI systems, and use the TUICast driver and reference TUI container images."
weight: 70
draft: true
---

## Writer brief

**Reader question:** "How do I run TUICast tests in our pipeline?"

**Cover:**

- Installing the driver in CI: the install script with a pinned version, or
  the container image.
- The two images, `castingcode/tuicast-driver` and
  `castingcode/tuicast-reference-tui`, their platforms, and how the driver
  image is used (JSON-RPC on standard input).
- Supplying credentials from the CI system's secret store.
- Saving wait-error screens and dumps as build artifacts; link
  [Diagnose failed waits](diagnose-failed-waits.md).
- A minimal GitHub Actions job. Base it on `.github/workflows/ci.yml` and
  `Taskfile.yml`.
- Network reachability: the CI runner must reach the application's SSH or
  Telnet port.

**Sources:** `design/containers.md`, `examples/reference/compose.yaml`,
`.github/workflows/ci.yml`.

**Length:** about 500 words.
