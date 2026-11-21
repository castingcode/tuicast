---
title: "Write Cucumber tests for terminal applications"
linkTitle: "Write Cucumber tests"
description: "Write behavior-driven Cucumber tests for terminal applications with TUICast, sharing one set of feature files across Go, Python, TypeScript, and Java."
weight: 60
draft: true
---

## Writer brief

**Reader question:** "How do I write readable BDD tests for our terminal
application that business analysts can review?"

**Cover:**

- Feature files describe what an operator does and sees; step definitions call
  TUICast. Show one scenario from `examples/features/login.feature` and its
  step definition in one language.
- What not to put in features: menu indexes, arrow-key sequences, timeouts,
  session IDs.
- Lifecycle hooks: connect in `Background`, always log out and close in an
  after-scenario hook, even after failures.
- Attaching terminal screens to the Cucumber report.
- Where each language's bindings live in `examples/`.

**Sources:** `design/cucumber.md`, `examples/features/`,
`examples/*/cucumber/`, `examples/reporting/`.

**Length:** about 600 words.
