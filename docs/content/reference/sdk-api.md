---
title: "SDK API reference"
linkTitle: "SDK API"
description: "Where to find the API reference for the TUICast Go, Python, TypeScript, and Java SDKs, and how each maps onto the driver protocol."
weight: 100
draft: true
---

## Writer brief

**Reader question:** "Where is the full list of classes and methods for my
SDK?"

**Cover:**

- A table with one row per SDK: package name and registry link, generated API
  documentation link, minimum language version, and README link.
  - Go: `github.com/castingcode/tuicast/sdk/go` on pkg.go.dev.
  - Python: `tuicast` on PyPI. Decide with engineering where generated docs
    will be hosted.
  - TypeScript: `@castingcode/tuicast` on npm. Decide whether to publish
    TypeDoc output.
  - Java: `dev.castingcode:tuicast` on Maven Central and javadoc.io.
- A short table mapping core operations across SDKs (launch, connect, open
  session, type, press, wait for text, close), so readers can translate
  examples between languages.
- Do not duplicate generated API documentation on this site.

**Sources:** `sdk/*/README.md`, `design/*-sdk.md`, the getting-started
guides.

**Length:** two tables.
