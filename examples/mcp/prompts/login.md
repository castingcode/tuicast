# Login workflow

Use the TUICast MCP tools to exercise the successful login workflow against
the `reference-ssh` profile.

1. List the available profiles and confirm that `reference-ssh` is available.
2. Connect to that profile and open a terminal session.
3. Wait for `LOGIN / AUTHENTICATION` before sending input.
4. Log in to the application with user ID `operator` and the application
   password.
5. Wait for `TERMINAL TEST SYSTEM`, then confirm that the screen contains
   `Authenticated as operator`.
6. Report the observed result.

Ask me for the application password when you need it. Type it with the
`parameter` set to `password`, and never repeat it in your reports or in files.

Always close the session and connection when finished, including when a step
fails. Do not connect to any profile other than `reference-ssh`.
