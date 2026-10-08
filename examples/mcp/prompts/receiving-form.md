# Receiving form workflow

Use the TUICast MCP tools to complete a receiving form in the
`reference-ssh` profile.

1. Connect, open a terminal session, and wait for the login screen.
2. Log in with application user ID `operator` and the application password.
3. From `TERMINAL TEST SYSTEM`, open `Forms and Input Fields` and wait for the
   `RECEIVING FORM` screen.
4. Submit these values:
   - Purchase order: `PO-10002341`
   - Item: `WIDGET-42`
   - Quantity: `25`
   - Location: `A-01-02`
   - Notes: `dock 3`
   - Urgent: enabled
5. Submit the form and wait for `RECEIPT ACCEPTED`.
6. Confirm that the result contains
   `PO-10002341 / WIDGET-42 / quantity 25 / A-01-02 / Urgent`, then report the
   observed result.

Ask me for the application password when you need it. Type it with the
`parameter` set to `password`, and never repeat it in your reports or in files.

Inspect the screen between navigation steps instead of assuming that input was
accepted. Always close the session and connection when finished, including
when a step fails. Do not connect to any profile other than `reference-ssh`.
