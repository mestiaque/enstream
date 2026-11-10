#!/usr/bin/env bash
# Run a Go command in the official Go container (no Go install needed on the host).
#   scripts/go.sh test ./...      scripts/go.sh build -o dist/enstream ./cmd/enstream
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$HOME/.cache/enstream-go/mod" "$HOME/.cache/enstream-go/build"
exec docker run --rm -i --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e GOMODCACHE=/gocache/mod -e GOCACHE=/gocache/build -e GOFLAGS="${GOFLAGS:-}" \
  -e CGO_ENABLED="${CGO_ENABLED:-0}" -e GOOS="${GOOS:-}" -e GOARCH="${GOARCH:-}" \
  -v "$HOME/.cache/enstream-go":/gocache -v "$ROOT":/src -w /src \
  ${ENSTREAM_DOCKER_ARGS:-} golang:1.25-bookworm go "$@"
