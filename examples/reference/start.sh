#!/bin/sh
set -eu

directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
docker compose --file "$directory/compose.yaml" up --build --detach
