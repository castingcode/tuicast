# Runnable examples

Examples are grouped by SDK language and share the same reference application
fixture and scenario names:

```text
examples/
  features/           shared, language-neutral Cucumber features
  reference/          shared SSH service and lifecycle scripts
  go/                 independent Go module using sdk/go
    cucumber/         Godog bindings for the shared features
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

The Cucumber features describe reference-application behavior without exposing
SDK calls or menu-navigation mechanics. Language-specific bindings should reuse
these files rather than copy them. Each scenario connects in its `Background`;
an after-scenario hook logs out when authenticated and always closes the
session, connection, and driver, including after failures.

Godog attaches a rendered terminal capture after changed outcome screens and
immediately after failures. CI publishes these captures in a Cucumber HTML
report. See [`design/cucumber.md`](../design/cucumber.md) for the feature-writing,
lifecycle, capture, and reporting conventions.

See [reference/README.md](reference/README.md) for a runnable walkthrough.
