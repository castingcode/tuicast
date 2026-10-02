# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o /out/tuicast-driver ./cmd/tuicast-driver

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tuicast-driver /usr/local/bin/tuicast-driver
ENTRYPOINT ["/usr/local/bin/tuicast-driver"]
