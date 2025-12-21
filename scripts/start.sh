#!/bin/sh
set -e

echo "=== IOTA SDK Startup ==="

# Start log collector in background
echo "[1/4] Starting log collector..."
collect_logs &

# Run database migrations
echo "[2/4] Running migrations..."
command migrate up
MIGRATE_EXIT=$?
echo "[2/4] Migrations completed with exit code: $MIGRATE_EXIT"

# Run database seeding
echo "[3/4] Running seed..."
command seed
SEED_EXIT=$?
echo "[3/4] Seed completed with exit code: $SEED_EXIT"

# Start the main server (exec replaces shell process)
echo "[4/4] Starting server..."
exec run_server
