#!/usr/bin/env bash
# Run a command in the Tauri build container shared with the sister apps (endl-build:
# Ubuntu 22.04 + Rust + Node + pnpm), with the desktop folder mounted at /src.
set -euo pipefail
DESKTOP="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${ENSTREAM_IMAGE:-endl-build}"
mkdir -p "$HOME/.cargo/registry" "$DESKTOP/target"
{ echo "root:x:0:0:root:/root:/bin/bash"; echo "builder:x:$(id -u):$(id -g):builder:/tmp/home:/bin/bash"; } >"$DESKTOP/target/.passwd"
exec docker run --rm -i --init ${ENSTREAM_DOCKER_ARGS:-} \
  --user "$(id -u):$(id -g)" -e HOME=/tmp/home -e CARGO_INCREMENTAL=0 -e CARGO_TARGET_DIR=/src/target \
  -v "$DESKTOP/target/.passwd":/etc/passwd:ro \
  -v "$DESKTOP":/src -v "$HOME/.cargo/registry":/opt/cargo/registry \
  -w /src "$IMAGE" bash -lc "mkdir -p /tmp/home && $*"
