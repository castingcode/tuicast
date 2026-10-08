#!/bin/sh
# Runs a command while the reference TUI serves SSH and Telnet, then stops both
# servers, mirroring the fixture steps in CI. Expects `task fixtures` to have
# built .bin/reference-tui and .bin/tuicast-driver.
#
# Usage: scripts/with-reference-fixture.sh command [argument...]
#
# Addresses come from TUICAST_REFERENCE_ADDRESS and
# TUICAST_REFERENCE_TELNET_ADDRESS. As in CI, fresh random passwords are
# generated for this run unless TUICAST_REFERENCE_PASSWORD and
# TUICAST_REFERENCE_APP_PASSWORD are already set. Twelve random bytes give 24
# hex characters, the limit of the reference application's login fields.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin="$root/.bin"

: "${TUICAST_REFERENCE_ADDRESS:=127.0.0.1:2222}"
: "${TUICAST_REFERENCE_TELNET_ADDRESS:=127.0.0.1:2323}"
if [ -z "${TUICAST_REFERENCE_PASSWORD:-}" ]; then
	TUICAST_REFERENCE_PASSWORD=$(openssl rand -hex 12)
fi
if [ -z "${TUICAST_REFERENCE_APP_PASSWORD:-}" ]; then
	TUICAST_REFERENCE_APP_PASSWORD=$(openssl rand -hex 12)
fi
# Always test the driver built from this checkout, as CI does.
TUICAST_DRIVER="$bin/tuicast-driver"
export TUICAST_REFERENCE_ADDRESS TUICAST_REFERENCE_TELNET_ADDRESS \
	TUICAST_REFERENCE_PASSWORD TUICAST_REFERENCE_APP_PASSWORD TUICAST_DRIVER

ssh_log="$bin/reference-ssh.log"
telnet_log="$bin/reference-telnet.log"
"$bin/reference-tui" --ssh-address "$TUICAST_REFERENCE_ADDRESS" >"$ssh_log" 2>&1 &
ssh_pid=$!
"$bin/reference-tui" --telnet-address "$TUICAST_REFERENCE_TELNET_ADDRESS" >"$telnet_log" 2>&1 &
telnet_pid=$!
trap 'kill "$ssh_pid" "$telnet_pid" 2>/dev/null || true' EXIT
trap 'exit 130' INT TERM

attempts=0
until grep -q 'SSH server listening' "$ssh_log" && grep -q 'Telnet server listening' "$telnet_log"; do
	if ! kill -0 "$ssh_pid" 2>/dev/null || ! kill -0 "$telnet_pid" 2>/dev/null || [ "$attempts" -ge 50 ]; then
		echo "The reference TUI fixtures did not start:" >&2
		cat "$ssh_log" "$telnet_log" >&2
		exit 1
	fi
	attempts=$((attempts + 1))
	sleep 0.1
done

"$@"
