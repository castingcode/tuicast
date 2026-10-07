# Explore and write a test

Use the TUICast MCP tools to explore the `reference-ssh` profile, then write an
automated TUICast test for the workflow you verified.

Write the test in the language of the TUICast SDK you use. Read its getting
started guide first, for example
[`docs/getting_started_python.md`](../../../docs/getting_started_python.md),
and use only the SDK API it documents.

The test should verify this behavior: after logging in with application user
ID `operator` and password `casting`, opening `Tables` from
`TERMINAL TEST SYSTEM`, and filtering the `WAREHOUSE ORDERS` table for `HOLD`,
opening an order shows its details with status `HOLD`.

1. Connect, open a session, and explore until you know the exact keys, text,
   and screen states the workflow needs. Inspect the screen after each step
   instead of assuming input was accepted.
2. Return to a known starting point by closing the session and opening a new
   one. Start recording, then repeat only the steps the test needs. Type the
   password with the `password` parameter so it is not stored in the
   recording.
3. Wait for every screen state the test will assert. Use `tuicast_wait` for
   assertions that need more than one condition or an exact line or cursor
   position, a stable period where the screen redraws after input, and
   `tuicast_wait_for_idle` only where no specific text marks readiness.
4. Stop the recording and translate each recorded step into the matching SDK
   call: `type` to typing text, `press` to pressing a key, `waitForText` and
   `wait` to the SDK's wait with an equivalent matcher, and `waitForIdle` to
   its idle wait. Keep the recorded timeouts and stable periods.
5. Read the connection address, SSH username, and passwords from environment
   variables or test configuration instead of embedding them in the test.
6. Report the recording you used and the test you wrote. Point out any step
   that relies on screen content likely to vary between runs.

Always close the session and connection when finished, including when a step
fails. Do not connect to any profile other than `reference-ssh`.
