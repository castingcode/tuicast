#!/bin/sh
# Prints one named secret from the workspace's Git-ignored .env.local file.
# tuicast-mcp runs this as its -secret-command, passing the variable name that
# a profile references, such as TUICAST_REFERENCE_PASSWORD, as the only
# argument, and reads the value from standard output.
set -eu

name=${1:-}
case "$name" in
	'' | [0-9]* | *[!A-Za-z0-9_]*)
		echo "invalid secret name: $name" >&2
		exit 2
		;;
esac

file="$(dirname "$0")/../../.env.local"
if [ ! -f "$file" ]; then
	echo "$file not found; copy .env.example to .env.local and set $name" >&2
	exit 1
fi

. "$file"
eval "value=\${$name:-}"
if [ -z "$value" ]; then
	echo "$name is not set in $file" >&2
	exit 1
fi
printf '%s\n' "$value"
