# Runnable examples

Examples are grouped by SDK language and share the same reference application
fixture and scenario names:

```text
examples/
  reference/          shared SSH service and lifecycle scripts
  go/                 independent Go module using sdk/go
    reference/        tests against the reference application
  python/             future Python SDK examples
  java/               future Java SDK examples
```

The language examples remain independent of the root implementation module.
That keeps their dependency graphs representative of what an SDK consumer
installs.

The Go examples demonstrate form and table workflows, stable waits for partial
screen updates, long-running operations, resize handling, structured cell
inspection, keys and modifiers, timeout diagnostics, concurrent sessions,
screen subscriptions, BELL/ENQ event subscriptions, and SSH/Telnet transport
parity.

See [reference/README.md](reference/README.md) for a runnable walkthrough.
