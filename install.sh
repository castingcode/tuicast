#!/bin/sh
set -eu

REPOSITORY="castingcode/tuicast"
BINDIR="${BINDIR:-./bin}"
COMPONENT="${TUICAST_COMPONENT:-driver}"
VERSION="${TUICAST_VERSION:-latest}"

usage() {
	cat <<EOF
Install TUICast from GitHub Releases.

Usage: $0 [-b DIR] [-c COMPONENT] [VERSION]

Options:
  -b, --bin-dir DIR       Installation directory (default: ./bin)
  -c, --component NAME   driver, mcp, reference-tui, or all (default: driver)
  -h, --help             Show this help

VERSION defaults to the latest release and may be written with or without a
leading "v". BINDIR, TUICAST_COMPONENT, and TUICAST_VERSION provide the same
settings through environment variables.
EOF
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		-b | --bin-dir)
			[ "$#" -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }
			BINDIR=$2
			shift 2
			;;
		-c | --component)
			[ "$#" -ge 2 ] || { echo "missing value for $1" >&2; exit 2; }
			COMPONENT=$2
			shift 2
			;;
		-h | --help)
			usage
			exit 0
			;;
		-*)
			echo "unknown option: $1" >&2
			usage >&2
			exit 2
			;;
		*)
			VERSION=$1
			shift
			[ "$#" -eq 0 ] || { echo "only one version may be specified" >&2; exit 2; }
			;;
	esac
done

case "$COMPONENT" in
	driver) BINARIES="tuicast-driver" ;;
	mcp) BINARIES="tuicast-mcp" ;;
	reference-tui) BINARIES="reference-tui" ;;
	all) BINARIES="tuicast-driver tuicast-mcp reference-tui" ;;
	*)
		echo "unsupported component: $COMPONENT" >&2
		echo "choose driver, mcp, reference-tui, or all" >&2
		exit 2
		;;
esac

download() {
	destination=$1
	url=$2
	if command -v curl >/dev/null 2>&1; then
		curl --fail --location --silent --show-error --output "$destination" "$url"
	elif command -v wget >/dev/null 2>&1; then
		wget --quiet --output-document="$destination" "$url"
	else
		echo "curl or wget is required" >&2
		return 1
	fi
}

download_stdout() {
	url=$1
	if command -v curl >/dev/null 2>&1; then
		curl --fail --location --silent --show-error "$url"
	elif command -v wget >/dev/null 2>&1; then
		wget --quiet --output-document=- "$url"
	else
		echo "curl or wget is required" >&2
		return 1
	fi
}

sha256() {
	file=$1
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$file" | awk '{print $1}'
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 "$file" | awk '{print $NF}'
	else
		echo "sha256sum, shasum, or openssl is required" >&2
		return 1
	fi
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	darwin | linux) ;;
	*) echo "unsupported operating system: $os" >&2; exit 1 ;;
esac

machine=$(uname -m)
case "$machine" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) echo "unsupported architecture: $machine" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
	echo "Finding the latest TUICast release..."
	release_json=$(download_stdout "https://api.github.com/repos/${REPOSITORY}/releases/latest")
	tag=$(printf '%s\n' "$release_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$tag" ] || { echo "could not determine the latest TUICast release" >&2; exit 1; }
else
	case "$VERSION" in
		v*) tag=$VERSION ;;
		*) tag="v$VERSION" ;;
	esac
fi

version=${tag#v}
archive_base="tuicast_${version}_${os}_${arch}"
archive_name="${archive_base}.tar.gz"
release_url="https://github.com/${REPOSITORY}/releases/download/${tag}"
tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t tuicast)
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup 0 1 2 15

echo "Downloading TUICast $tag for $os/$arch..."
download "$tmpdir/$archive_name" "$release_url/$archive_name"
download "$tmpdir/checksums.txt" "$release_url/checksums.txt"

expected=$(awk -v name="$archive_name" '$2 == name || $2 == ("*" name) { print $1; exit }' "$tmpdir/checksums.txt")
[ -n "$expected" ] || { echo "checksum not found for $archive_name" >&2; exit 1; }
actual=$(sha256 "$tmpdir/$archive_name")
[ "$actual" = "$expected" ] || { echo "checksum verification failed for $archive_name" >&2; exit 1; }

tar -xzf "$tmpdir/$archive_name" -C "$tmpdir"
mkdir -p "$BINDIR"
for binary in $BINARIES; do
	source_path="$tmpdir/$archive_base/$binary"
	[ -f "$source_path" ] || { echo "$binary was not found in $archive_name" >&2; exit 1; }
	cp "$source_path" "$BINDIR/$binary"
	chmod 0755 "$BINDIR/$binary"
	echo "Installed $BINDIR/$binary"
done
