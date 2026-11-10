#!/usr/bin/env bash
# Run ENStream from source with a throw-away data folder in .data/.
set -euo pipefail
cd "$(dirname "$0")/.."
if command -v go >/dev/null; then exec go run ./cmd/enstream -data .data -debug "$@"; fi
./scripts/go.sh build -o dist/enstream-dev ./cmd/enstream && exec ./dist/enstream-dev -data .data -debug "$@"
