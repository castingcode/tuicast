# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/reference-tui ./cmd/reference-tui

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install --no-install-recommends --yes vttest \
    && rm -rf /var/lib/apt/lists/*
ENV TERM=xterm-256color
COPY --from=build /out/reference-tui /usr/local/bin/reference-tui
USER nobody
ENTRYPOINT ["/usr/local/bin/reference-tui"]
