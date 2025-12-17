---
name: Database Connection
description: Connect to local or staging PostgreSQL database. Use when you need to inspect tables, run queries, check migration status, or debug database issues.
---

## Local Database

Extract variables from .env:

```bash
grep '^DB_' .env
```

Parse output, construct postgresql URL, then connect:

```bash
psql "postgresql://{DB_USER}:{DB_PASSWORD}@{DB_HOST}:{DB_PORT}/{DB_NAME}"
```

**Default local connection**:

```bash
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d iota_erp
```

## Staging Database

Extract public database URL from Railway:

```bash
railway variables -e staging -s db --kv | grep DATABASE_PUBLIC_URL
```

Connect using extracted URL:

```bash
psql <DATABASE_PUBLIC_URL>
```

**Example for IOTA SDK staging**:

```bash
# Get connection details from Railway
railway variables -e staging -s db --kv

# Connect (example)
PGPASSWORD=<password> psql -h <host> -U postgres -p <port> -d railway
```

## Helper Queries

Common diagnostics after connecting:

### Table Sizes

```sql
SELECT schemaname, tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;
```

### Active Connections

```sql
SELECT datname, count(*)
FROM pg_stat_activity
GROUP BY datname;
```

### Recent Migrations

```sql
-- Schema migrations (sql-migrate)
SELECT version, applied_at
FROM schema_migrations
ORDER BY applied_at DESC
LIMIT 5;
```

### Database Size

```sql
SELECT pg_size_pretty(pg_database_size(current_database()));
```

### Tenant Statistics

```sql
-- Count tenants
SELECT COUNT(*) as tenant_count FROM tenants WHERE deleted_at IS NULL;

-- Count users per tenant
SELECT t.name, COUNT(u.id) as user_count
FROM tenants t
LEFT JOIN users u ON u.tenant_id = t.id AND u.deleted_at IS NULL
WHERE t.deleted_at IS NULL
GROUP BY t.id, t.name
ORDER BY user_count DESC
LIMIT 10;
```

### Organization Statistics

```sql
-- Count organizations
SELECT COUNT(*) as org_count FROM organizations WHERE deleted_at IS NULL;

-- Organizations per tenant
SELECT t.name, COUNT(o.id) as org_count
FROM tenants t
LEFT JOIN organizations o ON o.tenant_id = t.id AND o.deleted_at IS NULL
WHERE t.deleted_at IS NULL
GROUP BY t.id, t.name
ORDER BY org_count DESC;
```

## Common Troubleshooting Queries

### Find Long-Running Queries

```sql
SELECT pid, now() - pg_stat_activity.query_start AS duration, query
FROM pg_stat_activity
WHERE state = 'active'
  AND now() - pg_stat_activity.query_start > interval '1 minute'
ORDER BY duration DESC;
```

### Check Table Bloat

```sql
SELECT tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as total_size,
       pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
       pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as indexes_size
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 20;
```

### Find Missing Indexes

```sql
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
  AND n_distinct > 100
  AND correlation < 0.1
ORDER BY n_distinct DESC
LIMIT 20;
```

### Check Lock Conflicts

```sql
SELECT blocked_locks.pid AS blocked_pid,
       blocked_activity.usename AS blocked_user,
       blocking_locks.pid AS blocking_pid,
       blocking_activity.usename AS blocking_user,
       blocked_activity.query AS blocked_statement,
       blocking_activity.query AS blocking_statement
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```

## Multi-Tenant Verification

### Verify Tenant Isolation

```sql
-- Check if tenant_id is consistently applied
SELECT table_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name NOT IN ('tenants', 'schema_migrations', 'sessions')
  AND table_name NOT LIKE 'pg_%'
  AND table_name NOT IN (
    SELECT table_name
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND column_name = 'tenant_id'
  );
```

### Verify Organization Isolation

```sql
-- Check if organization_id is applied where needed
SELECT table_name, column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND column_name = 'organization_id';
```

## Database Maintenance

### Vacuum Statistics

```sql
SELECT schemaname, tablename,
       last_vacuum, last_autovacuum,
       last_analyze, last_autoanalyze
FROM pg_stat_user_tables
ORDER BY last_autovacuum DESC NULLS LAST;
```

### Reindex Table

```sql
-- Check for bloated indexes
REINDEX TABLE table_name;
```

### Analyze Table

```sql
-- Update statistics
ANALYZE table_name;
```

## Connection Tips

### Using Environment Variables

```bash
# Export from .env
export $(grep '^DB_' .env | xargs)

# Connect using exported vars
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME
```

### Connection Pooler

```bash
# If using pgbouncer
psql "postgresql://$DB_USER:$DB_PASSWORD@localhost:6432/$DB_NAME"
```

### SSL Connections

```bash
# For production/staging with SSL
psql "postgresql://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=require"
```

## Safety Checks

### Before Making Changes

```sql
-- Always verify you're on the right database
SELECT current_database();

-- Check current schema
SELECT current_schema();

-- Verify tenant context if applicable
-- (Application-level check, not SQL)
```

### Backup Before Destructive Operations

```bash
# Dump specific table
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME -t table_name > backup.sql

# Dump entire database
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME > full_backup.sql
```

## Quick Reference

```bash
# Local connection
PGPASSWORD=postgres psql -h localhost -p 5432 -U postgres -d iota_erp

# Staging connection (Railway)
railway variables -e staging -s db --kv | grep DATABASE_PUBLIC_URL
psql <DATABASE_PUBLIC_URL>

# Run query from file
psql -h localhost -U postgres -d iota_erp -f query.sql

# Export to CSV
psql -h localhost -U postgres -d iota_erp -c "COPY (SELECT * FROM users) TO STDOUT WITH CSV HEADER" > users.csv

# List all tables
\dt

# Describe table
\d table_name

# List all indexes
\di

# Quit
\q
```
