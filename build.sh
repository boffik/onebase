#!/bin/bash
set -e
cd "$(dirname "$0")"

echo "=== onebase build ==="
echo ""

echo "[1/1] onebase  (CLI + сервер)..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o onebase ./cmd/onebase
echo "    OK"

echo ""
echo "Готово!"
echo "  ./onebase start  — запустит сервер и откроет в браузере."
