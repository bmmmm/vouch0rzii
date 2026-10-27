#!/usr/bin/env bash
# Assemble the static site for GitHub Pages into web/dist: the page files,
# Go's wasm_exec.js (must match the compiling toolchain), the wasm build of
# cmd/vouch0rzii-wasm, and the demo link (an admin state of the example
# roster, encoded by the CLI) in place of __DEMO__ in index.html. Run from
# the repo root; the output dir is gitignored.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT=web/dist
rm -rf "$OUT"
mkdir -p "$OUT"

cp web/index.html web/app.js web/style.css "$OUT"/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT"/   # Go >= 1.24 ships it under lib/wasm

DEMO="$(go run . -event events/congress2026.json -share admin -base '')"
perl -pi -e "s|__DEMO__|$DEMO|g" "$OUT/index.html"

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
GOOS=js GOARCH=wasm go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION}" \
  -o "$OUT/vouch0rzii.wasm" ./cmd/vouch0rzii-wasm

touch "$OUT/.nojekyll"   # Pages must not run Jekyll over the output

echo "built $OUT (version ${VERSION}):"
ls -l "$OUT"
