#!/bin/bash
# Rebuild Capacitarr: stops server, copies frontend assets, rebuilds Go binary, restarts
set -e

BACKEND_DIR="/home/ghent/src/workspaces/software/capacitarr/capacitarr/backend"
FRONTEND_DIR="/home/ghent/src/workspaces/software/capacitarr/capacitarr/frontend"
BINARY="/tmp/capacitarr"

echo "→ Stopping server..."
pkill -f '/tmp/capacitarr' 2>/dev/null || true
sleep 1

echo "→ Copying frontend assets..."
rm -rf "$BACKEND_DIR/frontend/dist"
cp -r "$FRONTEND_DIR/.output/public" "$BACKEND_DIR/frontend/dist"

echo "→ Building Go binary..."
cd "$BACKEND_DIR"
go build -o "$BINARY" .

echo "→ Starting server..."
"$BINARY" &
sleep 2

echo "✓ Capacitarr rebuilt and running on :8080"
