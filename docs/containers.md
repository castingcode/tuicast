# Container Images

TUICast provides separate Linux images for the JSON-RPC driver and the
reference terminal application. Both support amd64 and arm64. They retain the
binaries' stream-oriented interfaces rather than adding a network protocol:
the driver reads JSON-RPC from stdin and the reference application uses an
interactive terminal.

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

Publishing a GitHub Release runs `.github/workflows/release-images.yml`. It
builds both architectures and publishes semantic-version tags to:

- `ghcr.io/castingcode/tuicast-driver`
- `ghcr.io/castingcode/tuicast-reference-tui`

The reference image includes `vttest`, so every menu option is available. The
driver image is a minimal non-root image. Mount known-hosts files into the
driver container when using SSH's `knownHostsFile` verification option.
