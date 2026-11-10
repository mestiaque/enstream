#!/usr/bin/env bash
# Build ENStream for Linux and Windows into dist/. Uses a local Go when present, else Docker.
#   ./scripts/build.sh [version]
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always 2>/dev/null | sed 's/^v//' || echo dev)}"
LDFLAGS="-s -w -X enstream/internal/app.Version=${VERSION}"
GO=(go)
command -v go >/dev/null || GO=(./scripts/go.sh)
mkdir -p dist
for target in linux/amd64 linux/arm64 windows/amd64; do
  os="${target%/*}"; arch="${target#*/}"; ext=""; [ "$os" = windows ] && ext=".exe"
  echo "building ${os}/${arch}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "${GO[@]}" build -trimpath -ldflags "$LDFLAGS" -o "dist/enstream-${os}-${arch}${ext}" ./cmd/enstream
done
ls -la dist
