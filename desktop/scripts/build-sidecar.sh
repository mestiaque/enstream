#!/usr/bin/env bash
# Build the ENStream server for the desktop bundle: src-tauri/binaries/enstream-server-<triple>
# (Tauri's externalBin naming). Usage: build-sidecar.sh [linux|windows|all] [version]
set -euo pipefail
DESKTOP="$(cd "$(dirname "$0")/.." && pwd)"
ROOT="$(cd "$DESKTOP/.." && pwd)"
WHICH="${1:-linux}"
VERSION="${2:-$(sed -n 's/.*"version": "\(.*\)",/\1/p' "$DESKTOP/src-tauri/tauri.conf.json" | head -1)}"
OUT="$DESKTOP/src-tauri/binaries"
mkdir -p "$OUT"
GO="${GO:-go}"
if ! command -v "$GO" >/dev/null 2>&1; then GO="$ROOT/scripts/go.sh"; fi
build() { # goos goarch triple ext
  echo "building enstream-server for $3"
  (cd "$ROOT" && CGO_ENABLED=0 GOOS="$1" GOARCH="$2" "$GO" build -trimpath -ldflags "-s -w -X enstream/internal/app.Version=$VERSION" \
    -o "desktop/src-tauri/binaries/enstream-server-$3$4" ./cmd/enstream)
}
case "$WHICH" in
  linux) build linux amd64 x86_64-unknown-linux-gnu "" ;;
  windows) build windows amd64 x86_64-pc-windows-msvc .exe ;;
  all) build linux amd64 x86_64-unknown-linux-gnu ""; build windows amd64 x86_64-pc-windows-msvc .exe ;;
  *) echo "usage: $0 [linux|windows|all] [version]" >&2; exit 2 ;;
esac
ls -l "$OUT"
