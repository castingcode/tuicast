# TypeScript SDK

The independently publishable `@castingcode/tuicast` package in `sdk/typescript`
is an ESM library for Node.js 24 (the current LTS). It has no runtime
dependencies. `Driver.launch()` starts `tuicast-driver`, verifies stdio JSON-RPC
protocol version 1, correlates concurrent responses, and owns process shutdown.

## API

`Driver.connect()` accepts discriminated `SSHConfig` and `TelnetConfig` values;
SSH authentication and exactly one host-key policy are validated locally.
Connections open VT220 80x24 sessions by default, or callers can select
`xterm-256color`, dimensions, and ENQ answerback. `Session` supports text and
byte input, named keys/modifiers, resizing, detached screens, recursive
matchers, stable and idle waits, and screen/event subscriptions as
`AsyncIterable`s. Screen updates coalesce; terminal events use an ordered
64-item bounded queue. All operations accept `AbortSignal` and millisecond
timeouts. `RPCError` and `WaitError` preserve structured driver diagnostics.
All lifecycle `close()` methods are idempotent.

```ts
const driver = await Driver.launch({ path: "/path/to/tuicast-driver" });
const connection = await driver.connect({
  protocol: "ssh", address: "host:22", username: "operator",
  password: "...", knownHostsFile: "/home/me/.ssh/known_hosts"
});
const session = await connection.openSession({ terminal: "xterm-256color" });
const screen = await session.waitForText("READY", { stableFor: 100 });
```

## Development and packaging

Use Node 24 and npm. With nvm, select the repository version before installing
dependencies:

```sh
cd sdk/typescript
nvm install
nvm use
npm ci
```

Then run `npm run format:check`, `npm run typecheck`,
`npm run test:coverage`, and `npm run pack:check`. Tests use Node's test runner
and built-in coverage and include the shared screen-query fixtures,
protocol/process failures, out-of-order concurrency, cancellation, errors,
notification buffering, and cleanup.

From `examples/typescript`, run `npm ci`, `npm run typecheck`, then build the Go
driver/reference fixture and run `TUICAST_DRIVER=... npm run reference` and
`npm run cucumber`. The reference suite contains twelve distinct integration
tests mirroring every Go reference behavior. The latter executes
`examples/features` directly. Run
`npm run cucumber:report` to write `cucumber-reports/json/typescript.json`, then
invoke `node examples/reporting/generate-cucumber-report.mjs
cucumber-reports/json <report-directory>`. Cucumber captures changed
outcome screens as inline SVG and captures SVG plus text immediately on failure;
successful input steps are not captured.

`npm pack --dry-run` validates the eventual npm artifact.

## Publishing

npm publication uses trusted publishing through
`.github/workflows/publish-typescript.yml`; the workflow does not use an npm
access token. Configure the npm trusted publisher for `@castingcode/tuicast`
with organization `castingcode`, repository `tuicast`, workflow
`publish-typescript.yml`, environment `npm`, and permission to run
`npm publish`. Configure the matching GitHub environment with required
reviewers before automated releases. Once trusted publishing works, set the
package's publishing access to require two-factor authentication and disallow
tokens.

The package version in `package.json` must match a tag named
`sdk/typescript/v<version>`. The workflow runs the complete CI suite, checks
the tag and package versions, builds the package, and publishes from a
GitHub-hosted runner using a short-lived OIDC credential. The public package
also receives npm provenance automatically.

Trusted publishers can only be configured after the package exists. Bootstrap
`@castingcode/tuicast` once from `sdk/typescript` with an interactive npm login,
account two-factor authentication, and `npm publish --access public`. No
bypass-2FA automation token is required. Because package versions cannot be
republished, automated releases must use a version newer than the bootstrap
version.
