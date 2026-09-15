# Shared Cucumber Examples

## Purpose and ownership

The feature files under `examples/features/` are the language-neutral behavior
specification for the reference TUI. Every SDK implementation should bind its
own step definitions to this one feature set. Features must not be copied into
language directories because copies would allow the SDK examples to describe
different application behavior.

The Go bindings live under `examples/go/cucumber/` and use Godog. Godog is an
implementation detail of the Go example, not a dialect that features may rely
on. A Python, Java, or other Cucumber implementation should be able to execute
the same files without changing their wording.

## Feature language and specificity

Features describe user intent and observable reference-application behavior.
Use the vocabulary visible to an operator: entering a username or password,
selecting a named menu option, pressing a named key, reaching a named screen,
and observing text or terminal presentation.

Do not expose automation mechanics in a feature. In particular, features must
not contain menu indexes, repeated arrow-key navigation, focus traversal needed
only by the implementation, driver methods, session identifiers, polling,
timeouts, stable-screen delays, or SDK data structures. For example:

```gherkin
When I select the "Unicode" menu option
Then the application is on the "Unicode" screen
```

The language binding owns how to reach that menu item and how to recognize the
screen. This keeps features stable if navigation changes and lets every SDK use
its idiomatic API.

Use exact text, zero-based coordinates, palette indexes, and attributes when
those details are the behavior under test, as they are for terminal rendering.
Do not add such details merely to help a step implementation find an element.
Prefer a named application state when the exact location is not itself part of
the contract.

The login feature enters credentials explicitly because authentication is its
subject. Other features use `Given I am logged in` in their `Background` so
they describe only their own behavior. Keep scenarios independent and focused
on one behavior or one coherent presentation state.

## Lifecycle

Connection setup is an explicit background precondition. The after-scenario
hook performs a best-effort application logout whenever the scenario reached
an authenticated state, then always closes the session, connection, and driver.
Cleanup runs for passing and failing scenarios. An unsuccessful login has no
authenticated state to log out and disconnects directly.

Navigation and assertions must finish before cleanup begins. Failure evidence
must therefore be captured in an after-step hook, not after logout.

## Terminal captures

The Go binding renders detached terminal cells as an inline SVG carried by a
`text/html` Godog attachment. The rendering preserves screen position, indexed
foreground and background colors, bold, underline, reverse, conceal, and cursor
position without depending on a desktop or a platform screenshot utility.

Captures follow these rules:

- After a successful outcome (`Then`) step, attach the screen only when its
  revision differs from the last attached revision. Several assertions against
  one unchanged screen therefore produce one capture.
- After any failed step, attach the current rendered screen and its plain-text
  contents immediately, before cleanup, even if that revision was attached
  earlier.
- Do not capture successful input steps. Besides reducing noise, this avoids
  recording credentials during normal runs. A failed input step still follows
  the failure rule, so fixtures must use deterministic, non-sensitive test data.
- Do not capture routine logout. Logout is lifecycle cleanup rather than the
  behavior under test. A future logout scenario may assert and capture it as an
  ordinary outcome.

Attachments are diagnostic evidence, not assertions. A scenario passes because
its steps verify the screen's text, position, colors, or attributes—not because
a capture was produced.

## CI reports

CI runs the ordinary Go reference examples and Cucumber examples separately.
The Cucumber run uses Godog's `cucumber` formatter to retain attachments in a
JSON report. `multiple-cucumber-html-reporter` 4.3.0 converts that report to an
HTML report directory. It is an actively maintained, development-only,
MIT-licensed package with no known dependency vulnerabilities at the time of
adoption and is pinned by `examples/reporting/package-lock.json`.

Report generation and artifact upload use `if: always()` so evidence from a
failed Cucumber run remains available. The `cucumber-reports` artifact contains
both the source JSON and rendered HTML directory. Reports may contain application
screen contents; only use this capture policy with deterministic test data, and
review redaction requirements before adapting it to another application.
