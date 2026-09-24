# `@castingcode/tuicast-cli`

Installs the TUICast terminal automation driver and reference TUI for Linux,
macOS, and Windows on x64 and ARM64 systems.

Install it globally:

```sh
npm install --global @castingcode/tuicast-cli
```

The `tuicast-driver` and `reference-tui` commands will then be available on
`PATH`. The package downloads the matching binaries from the TUICast GitHub
release and verifies their SHA-256 checksums during installation.

The TypeScript SDK remains the separate
[`@castingcode/tuicast`](https://www.npmjs.com/package/@castingcode/tuicast)
package.
