#!/bin/bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if ! command -v node >/dev/null 2>&1; then
  echo "Error: node is not installed."
  echo "Install Node.js (>= 18) and re-run this script."
  exit 1
fi

if ! command -v pnpm >/dev/null 2>&1; then
  if command -v corepack >/dev/null 2>&1; then
    corepack enable
    corepack prepare pnpm@10.19.0 --activate
  else
    echo "Error: pnpm is not installed and corepack is unavailable."
    echo "Install pnpm and re-run this script."
    exit 1
  fi
fi

cd "$REPO_ROOT"
pnpm install --frozen-lockfile
pnpm exec tailwindcss --help >/dev/null
echo "TailwindCSS installed (Tailwind v4 via pnpm)."
