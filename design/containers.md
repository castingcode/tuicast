# Container Images

TUICast provides separate Linux images for the JSON-RPC driver and the
reference terminal application. Both support amd64 and arm64. The driver reads
JSON-RPC from stdin. The reference application can use an interactive terminal
or listen for SSH or Telnet connections.

## Running locally

Build and run the native-platform driver:

```sh
docker build -f docker/driver.Dockerfile -t tuicast-driver .
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"driver.ping"}' \
  '{"jsonrpc":"2.0","id":2,"method":"driver.shutdown"}' \
  | docker run --rm -i tuicast-driver
```

Build and run the reference application with a TTY:

```sh
docker build -f docker/reference-tui.Dockerfile -t tuicast-reference-tui .
docker run --rm -it tuicast-reference-tui
```

Run the same image as the SSH example fixture:

```sh
docker run --rm -p 2222:2222 \
  tuicast-reference-tui --ssh-address 0.0.0.0:2222
```

The convenience Compose fixture and Go SDK example live under `examples/`.

Use Buildx when a local multi-platform output is required. A multi-platform
result must be pushed to a registry (or exported as an OCI archive); Docker's
classic local image store cannot load a multi-platform image directly.

```sh
docker buildx build --platform linux/amd64,linux/arm64 \
  -f docker/driver.Dockerfile -t registry.example/tuicast-driver:dev --push .
docker buildx build --platform linux/amd64,linux/arm64 \
  -f docker/reference-tui.Dockerfile \
  -t registry.example/tuicast-reference-tui:dev --push .
```

## Releases

Pushing a semantic-version tag such as `v1.2.3` runs
`.github/workflows/release.yml`. After the test and static-analysis checks pass,
it creates a GitHub Release with checksummed binaries and publishes Linux amd64
and arm64 image tags to:

- `ghcr.io/castingcode/tuicast-driver`
- `ghcr.io/castingcode/tuicast-reference-tui`

```sh
git tag v1.2.3
git push origin v1.2.3
```

The reference image includes `vttest`, so every menu option is available. The
driver image is a minimal non-root image. Mount known-hosts files into the
driver container when using SSH's `knownHostsFile` verification option.

Release archives contain both `tuicast-driver` and `reference-tui`, the
OpenRPC schema, and the project license. Archives are produced for Linux,
macOS, and Windows on amd64 and arm64.
