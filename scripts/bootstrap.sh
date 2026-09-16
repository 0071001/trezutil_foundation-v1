#!/usr/bin/env bash
set -euo pipefail

echo "Trezutil foundation bootstrap"

go version

docker --version

docker compose version

make up

echo "Infrastructure started."
