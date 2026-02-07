#!/usr/bin/env bash
# Update gorp_migrations after squashing changes-1770389268 and changes-1770469595 into one.
# Run from repo root with .env loaded (e.g. just db migrate status first to ensure DB is up).
#
# Usage:
#   cd /path/to/iota-sdk && set -a && source .env && set +a && bash scripts/squash-migrations-1770469595.sh
# Or if DATABASE_URL is set:
#   bash scripts/squash-migrations-1770469595.sh

set -euo pipefail

if [[ -n "${DATABASE_URL:-}" ]]; then
  PSQL_CMD=(psql "$DATABASE_URL")
else
  export PGPASSWORD="${DB_PASSWORD:-postgres}"
  PSQL_CMD=(psql -h "${DB_HOST:-localhost}" -p "${DB_PORT:-5432}" -U "${DB_USER:-postgres}" -d "${DB_NAME:-iota_erp}")
fi

# Remove both original migration records; mark the squashed one as applied
"${PSQL_CMD[@]}" -v ON_ERROR_STOP=1 <<'SQL'
DELETE FROM gorp_migrations
WHERE id IN ('changes-1770389268', 'changes-1770469595');

INSERT INTO gorp_migrations (id, applied_at)
VALUES ('changes-1770469595', NOW())
ON CONFLICT (id) DO NOTHING;
SQL

echo "Done: gorp_migrations updated for squashed migration changes-1770469595."
