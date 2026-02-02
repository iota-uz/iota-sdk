#!/usr/bin/env bash
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT/modules/bichat/presentation/web"
pnpm run build:css
pnpm run build
pnpm run build:watch &
WATCHER_PID=$!
trap 'kill $WATCHER_PID 2>/dev/null' EXIT

cd "$ROOT"
air
