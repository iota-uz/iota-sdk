#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NAME="${1:-}"

if [ -z "$NAME" ]; then
  echo "Usage: scripts/applet/dev.sh <name>" >&2
  exit 2
fi

cd "$ROOT"

if ! command -v air >/dev/null 2>&1; then
  echo "Missing 'air' in PATH." >&2
  echo "Install: go install github.com/air-verse/air@latest" >&2
  exit 1
fi

if [ ! -f "$ROOT/dist/index.mjs" ]; then
  echo "Missing SDK build artifacts under dist/. Running pnpm install at repo root..." >&2
  pnpm -C "$ROOT" install --prefer-frozen-lockfile
fi

APPLET_JSON="$ROOT/scripts/applets.json"
if [ ! -f "$APPLET_JSON" ]; then
  echo "Missing scripts/applets.json" >&2
  exit 1
fi

eval "$(
  node - <<'NODE' "$NAME" "$APPLET_JSON"
const [name, file] = process.argv.slice(2)
const fs = require('fs')
const data = JSON.parse(fs.readFileSync(file, 'utf8'))
const applet = (data.applets || []).find((a) => a.name === name)
if (!applet) {
  console.error(`Unknown applet: ${name}`)
  process.exit(2)
}
const viteDir = applet.viteDir
const basePath = applet.basePath
const vitePort = applet.vitePort
const entryModule = applet.entryModule
if (!viteDir || !basePath || !vitePort || !entryModule) {
  console.error(`Invalid registry entry for ${name}`)
  process.exit(2)
}
process.stdout.write(`APPLET_VITE_DIR=${JSON.stringify(viteDir)}\n`)
process.stdout.write(`APPLET_BASE_PATH=${JSON.stringify(basePath)}\n`)
process.stdout.write(`APPLET_VITE_PORT=${JSON.stringify(vitePort)}\n`)
process.stdout.write(`APPLET_ENTRY_MODULE=${JSON.stringify(entryModule)}\n`)
NODE
)"

VITE_DIR="$ROOT/$APPLET_VITE_DIR"
if [ ! -d "$VITE_DIR" ]; then
  echo "Vite dir not found: $VITE_DIR" >&2
  exit 1
fi

if [ ! -d "$VITE_DIR/node_modules" ]; then
  pnpm -C "$VITE_DIR" install --prefer-frozen-lockfile
fi

export APPLET_ASSETS_BASE="$APPLET_BASE_PATH/assets/"
export APPLET_VITE_PORT="$APPLET_VITE_PORT"

UPPER="$(echo "$NAME" | tr '[:lower:]-' '[:upper:]_')"
export "IOTA_APPLET_DEV_${UPPER}=1"
export "IOTA_APPLET_VITE_URL_${UPPER}=http://localhost:${APPLET_VITE_PORT}"
export "IOTA_APPLET_ENTRY_${UPPER}=${APPLET_ENTRY_MODULE}"
export "IOTA_APPLET_CLIENT_${UPPER}=/@vite/client"

IOTA_PORT="${IOTA_PORT:-3200}"
echo "Applet: $NAME"
echo "URL: http://localhost:${IOTA_PORT}${APPLET_BASE_PATH}"

cleanup() {
  pids="$(jobs -pr 2>/dev/null || true)"
  if [ -n "${pids:-}" ]; then
    kill ${pids} 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

pnpm -C "$VITE_DIR" run dev:embedded &
air &

wait
