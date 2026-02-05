#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NAME="${1:-}"

if [ -z "$NAME" ]; then
  echo "Usage: scripts/applet/rpc-gen.sh <name>" >&2
  exit 2
fi

if [[ ! "$NAME" =~ ^[A-Za-z][A-Za-z0-9_-]*$ ]]; then
  echo "Invalid applet name: $NAME" >&2
  exit 2
fi

cd "$ROOT"

TYPE_NAME="$(
  node - <<'NODE' "$NAME"
const name = process.argv[2] || ''
const pascal = name
  .split(/[-_]/g)
  .filter(Boolean)
  .map((p) => p[0].toUpperCase() + p.slice(1))
  .join('')
process.stdout.write(`${pascal}RPC`)
NODE
)"

ROUTER_PKG="modules/${NAME}/rpc"
OUT="modules/${NAME}/presentation/web/src/rpc.generated.ts"

GOTOOLCHAIN=auto go run ./cmd/applet-rpc-typegen --router-pkg "$ROUTER_PKG" --out "$OUT" --type-name "$TYPE_NAME"
