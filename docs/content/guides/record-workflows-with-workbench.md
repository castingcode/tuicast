---
title: "Record terminal workflows with Workbench"
linkTitle: "Record with Workbench"
description: "Use TUICast Workbench in a browser to drive a live terminal session, choose assertions, and download a replayable, language-neutral recording."
weight: 50
draft: true
---

## Writer brief

**Reader question:** "Can I record a workflow by using the application instead
of writing code first?"

**Cover:**

- Starting the driver with `-ui-address 127.0.0.1:0 -workbench`, and why it
  only works on loopback addresses.
- Recording: starting takes exclusive control of the session; the user types,
  presses keys, and selects screen text to turn into a wait.
- Why assertions are chosen explicitly rather than inferred (dynamic data on
  warehouse screens).
- Parameterized input for passwords and test data.
- Replaying and downloading the recording; link
  [Recording format](../reference/recording-format.md).
- Current limits: replay targets the original session only.
- Screenshots of the Workbench UI, each with descriptive alt text.

**Sources:** `design/workbench.md`, `design/driver-ui.md`.

**Length:** about 500 words plus screenshots.
