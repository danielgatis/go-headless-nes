#!/usr/bin/env bash
# Build the WASM binary and copy Go's JS support shim alongside it, so the
# whole example can be served as static files.
set -euo pipefail
cd "$(dirname "$0")"

GOOS=js GOARCH=wasm go build -o nes.wasm .

# wasm_exec.js ships with the Go toolchain. Its location moved to
# lib/wasm in Go 1.24+; fall back to the old misc/wasm path.
GOROOT="$(go env GOROOT)"
if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then
	cp "$GOROOT/lib/wasm/wasm_exec.js" .
else
	cp "$GOROOT/misc/wasm/wasm_exec.js" .
fi

echo "built nes.wasm — now serve this directory, e.g.:"
echo "  go run ./serve   (or any static HTTP server) and open http://localhost:8080"
