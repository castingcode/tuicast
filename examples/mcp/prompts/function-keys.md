# Function keys and modifiers

Use the TUICast MCP tools to demonstrate named keys and modifiers against the
`reference-ssh` profile.

1. Connect, open a terminal session, and log in with application user ID
   `operator` and password `casting`.
2. From `TERMINAL TEST SYSTEM`, open `Function Keys` and wait for the
   `FUNCTION KEYS` screen.
3. Press F5 and confirm that the screen reports `Last: f5` and
   `Modifiers: none`.
4. Press Control+C and confirm that the screen reports `Last: ctrl+c` and
   `Modifiers: control`.
5. Report both observed results.

Inspect the screen after each key press. Always close the session and
connection when finished, including when a step fails. Do not connect to any
profile other than `reference-ssh`.
