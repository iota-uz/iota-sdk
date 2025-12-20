# JavaScript Runtime - Database Schema

## Overview

The JavaScript Runtime database schema uses PostgreSQL with multi-tenant isolation, referential integrity, and performance indexes. All tables include `tenant_id` for tenant isolation and follow IOTA SDK migration patterns using sql-migrate.

## Tables

### 1. scripts

Main table storing script definitions with metadata, resource limits, and trigger configuration.

```sql
CREATE TABLE scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    source TEXT NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('scheduled', 'http', 'event', 'oneoff', 'embedded')),
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'disabled', 'archived')),

    -- Resource limits (stored as JSONB for flexibility)
    resource_limits JSONB NOT NULL DEFAULT '{
        "max_execution_time_ms": 30000,
        "max_memory_bytes": 67108864,
        "max_concurrent_runs": 5,
        "max_api_calls_per_minute": 60,
        "max_output_size_bytes": 1048576
    }'::jsonb,

    -- Trigger configuration
    cron_expression VARCHAR(100),
    http_path VARCHAR(500),
    http_methods TEXT[] DEFAULT '{}',
    event_types TEXT[] DEFAULT '{}',

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,
    tags TEXT[] DEFAULT '{}',

    -- Audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,

    -- Constraints
    CONSTRAINT scripts_name_tenant_unique UNIQUE (tenant_id, name),
    CONSTRAINT scripts_http_path_tenant_unique UNIQUE (tenant_id, http_path),
    CONSTRAINT scripts_cron_required CHECK (
        (type = 'scheduled' AND cron_expression IS NOT NULL) OR type != 'scheduled'
    ),
    CONSTRAINT scripts_http_path_required CHECK (
        (type = 'http' AND http_path IS NOT NULL) OR type != 'http'
    ),
    CONSTRAINT scripts_event_types_required CHECK (
        (type = 'event' AND array_length(event_types, 1) > 0) OR type != 'event'
    )
);

-- Indexes for performance
CREATE INDEX idx_scripts_tenant_id ON scripts(tenant_id);
CREATE INDEX idx_scripts_tenant_status ON scripts(tenant_id, status);
CREATE INDEX idx_scripts_tenant_type_status ON scripts(tenant_id, type, status);
CREATE INDEX idx_scripts_http_path ON scripts(tenant_id, http_path) WHERE http_path IS NOT NULL;
CREATE INDEX idx_scripts_event_types ON scripts USING GIN(event_types) WHERE array_length(event_types, 1) > 0;
CREATE INDEX idx_scripts_cron_expression ON scripts(tenant_id, cron_expression) WHERE cron_expression IS NOT NULL;
CREATE INDEX idx_scripts_metadata ON scripts USING GIN(metadata);
CREATE INDEX idx_scripts_tags ON scripts USING GIN(tags);
CREATE INDEX idx_scripts_created_at ON scripts(tenant_id, created_at DESC);

-- Full-text search index for script discovery
CREATE INDEX idx_scripts_fulltext ON scripts USING GIN(
    to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, ''))
);

COMMENT ON TABLE scripts IS 'User-defined JavaScript scripts with trigger configuration';
COMMENT ON COLUMN scripts.type IS 'Script trigger type: scheduled (cron), http (endpoint), event (domain event), oneoff (manual), embedded (programmatic)';
COMMENT ON COLUMN scripts.status IS 'Script lifecycle status: draft, active, paused, disabled, archived';
COMMENT ON COLUMN scripts.resource_limits IS 'Execution resource constraints (timeout, memory, concurrency, rate limits)';
COMMENT ON COLUMN scripts.cron_expression IS 'Cron schedule (required for scheduled scripts)';
COMMENT ON COLUMN scripts.http_path IS 'HTTP endpoint path (required for http scripts)';
COMMENT ON COLUMN scripts.http_methods IS 'Allowed HTTP methods for endpoint (GET, POST, PUT, DELETE)';
COMMENT ON COLUMN scripts.event_types IS 'Domain event types to trigger on (required for event scripts)';
```

### 2. script_executions

Execution history with input/output, status, metrics, and error tracking.

```sql
CREATE TABLE script_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout', 'cancelled')),

    -- Trigger information
    trigger_type VARCHAR(50) NOT NULL CHECK (trigger_type IN ('cron', 'http', 'event', 'manual', 'api')),
    trigger_data JSONB DEFAULT '{}'::jsonb,

    -- Input/Output
    input JSONB DEFAULT '{}'::jsonb,
    output JSONB,
    error TEXT,

    -- Metrics
    metrics JSONB DEFAULT '{
        "duration_ms": 0,
        "memory_used_bytes": 0,
        "api_call_count": 0,
        "database_query_count": 0
    }'::jsonb,

    -- Timestamps
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Indexes for performance
CREATE INDEX idx_executions_script_id ON script_executions(script_id);
CREATE INDEX idx_executions_tenant_id ON script_executions(tenant_id);
CREATE INDEX idx_executions_tenant_script ON script_executions(tenant_id, script_id);
CREATE INDEX idx_executions_tenant_status ON script_executions(tenant_id, status);
CREATE INDEX idx_executions_tenant_started_at ON script_executions(tenant_id, started_at DESC);
CREATE INDEX idx_executions_trigger_type ON script_executions(tenant_id, trigger_type);
CREATE INDEX idx_executions_pending ON script_executions(tenant_id, status) WHERE status = 'pending';
CREATE INDEX idx_executions_running ON script_executions(tenant_id, status) WHERE status = 'running';

COMMENT ON TABLE script_executions IS 'Script execution history with input/output and metrics';
COMMENT ON COLUMN script_executions.status IS 'Execution status: pending, running, completed, failed, timeout, cancelled';
COMMENT ON COLUMN script_executions.trigger_type IS 'What triggered execution: cron, http, event, manual, api';
COMMENT ON COLUMN script_executions.trigger_data IS 'Trigger context (event payload, HTTP request, etc.)';
COMMENT ON COLUMN script_executions.metrics IS 'Execution metrics (duration, memory, API calls, DB queries)';
```

### 3. script_versions

Version history for script source code changes (audit trail).

```sql
CREATE TABLE script_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    source TEXT NOT NULL,
    change_description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,

    -- Constraints
    CONSTRAINT script_versions_unique UNIQUE (script_id, version_number)
);

-- Indexes for performance
CREATE INDEX idx_versions_script_id ON script_versions(script_id);
CREATE INDEX idx_versions_tenant_id ON script_versions(tenant_id);
CREATE INDEX idx_versions_created_at ON script_versions(script_id, created_at DESC);

COMMENT ON TABLE script_versions IS 'Immutable audit trail of script source code changes';
COMMENT ON COLUMN script_versions.version_number IS 'Incrementing version number (1, 2, 3, ...)';
COMMENT ON COLUMN script_versions.change_description IS 'Human-readable description of changes';
```

### 4. script_event_subscriptions

Event-triggered script registrations (for fast event-to-script lookup).

```sql
CREATE TABLE script_event_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT script_event_subscriptions_unique UNIQUE (script_id, event_type)
);

-- Indexes for performance
CREATE INDEX idx_event_subs_tenant_event ON script_event_subscriptions(tenant_id, event_type) WHERE is_active = true;
CREATE INDEX idx_event_subs_script_id ON script_event_subscriptions(script_id);

COMMENT ON TABLE script_event_subscriptions IS 'Fast lookup table for event-triggered scripts';
COMMENT ON COLUMN script_event_subscriptions.event_type IS 'Domain event type (e.g., user.created, payment.processed)';
COMMENT ON COLUMN script_event_subscriptions.is_active IS 'Whether subscription is currently active';
```

### 5. script_scheduled_jobs

Cron job state tracking (next run time, last run, status).

```sql
CREATE TABLE script_scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL UNIQUE REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    cron_expression VARCHAR(100) NOT NULL,
    timezone VARCHAR(100) DEFAULT 'UTC',
    next_run_at TIMESTAMPTZ NOT NULL,
    last_run_at TIMESTAMPTZ,
    last_run_status VARCHAR(50),
    is_running BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_scheduled_jobs_next_run ON script_scheduled_jobs(next_run_at) WHERE is_running = false;
CREATE INDEX idx_scheduled_jobs_tenant_id ON script_scheduled_jobs(tenant_id);
CREATE INDEX idx_scheduled_jobs_script_id ON script_scheduled_jobs(script_id);

COMMENT ON TABLE script_scheduled_jobs IS 'Cron scheduler state for scheduled scripts';
COMMENT ON COLUMN script_scheduled_jobs.next_run_at IS 'Calculated next execution time based on cron expression';
COMMENT ON COLUMN script_scheduled_jobs.last_run_status IS 'Status of last execution (completed, failed, timeout)';
COMMENT ON COLUMN script_scheduled_jobs.is_running IS 'Whether script is currently executing (prevent overlapping runs)';
```

### 6. script_http_endpoints

HTTP endpoint routing table (for fast path-to-script lookup).

```sql
CREATE TABLE script_http_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL UNIQUE REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    http_path VARCHAR(500) NOT NULL,
    http_methods TEXT[] NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT script_http_endpoints_path_unique UNIQUE (tenant_id, http_path)
);

-- Indexes for performance
CREATE INDEX idx_http_endpoints_path ON script_http_endpoints(tenant_id, http_path) WHERE is_active = true;
CREATE INDEX idx_http_endpoints_script_id ON script_http_endpoints(script_id);

COMMENT ON TABLE script_http_endpoints IS 'Fast lookup table for HTTP endpoint scripts';
COMMENT ON COLUMN script_http_endpoints.http_path IS 'URL path (e.g., /api/scripts/my-handler)';
COMMENT ON COLUMN script_http_endpoints.http_methods IS 'Allowed HTTP methods (GET, POST, PUT, DELETE)';
```

### 7. script_event_dead_letters

Failed event-triggered executions for retry or manual review.

```sql
CREATE TABLE script_event_dead_letters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    execution_id UUID REFERENCES script_executions(id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    error TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_dead_letters_tenant_id ON script_event_dead_letters(tenant_id);
CREATE INDEX idx_dead_letters_script_id ON script_event_dead_letters(script_id);
CREATE INDEX idx_dead_letters_created_at ON script_event_dead_letters(tenant_id, created_at DESC);
CREATE INDEX idx_dead_letters_retry_count ON script_event_dead_letters(retry_count) WHERE retry_count < 3;

COMMENT ON TABLE script_event_dead_letters IS 'Failed event-triggered executions for retry or review';
COMMENT ON COLUMN script_event_dead_letters.retry_count IS 'Number of retry attempts (max 3)';
```

## Migrations

### Up Migration

```sql
-- +migrate Up
-- +migrate StatementBegin

-- Create scripts table
CREATE TABLE scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    source TEXT NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('scheduled', 'http', 'event', 'oneoff', 'embedded')),
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'paused', 'disabled', 'archived')),
    resource_limits JSONB NOT NULL DEFAULT '{
        "max_execution_time_ms": 30000,
        "max_memory_bytes": 67108864,
        "max_concurrent_runs": 5,
        "max_api_calls_per_minute": 60,
        "max_output_size_bytes": 1048576
    }'::jsonb,
    cron_expression VARCHAR(100),
    http_path VARCHAR(500),
    http_methods TEXT[] DEFAULT '{}',
    event_types TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}'::jsonb,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT scripts_name_tenant_unique UNIQUE (tenant_id, name),
    CONSTRAINT scripts_http_path_tenant_unique UNIQUE (tenant_id, http_path),
    CONSTRAINT scripts_cron_required CHECK (
        (type = 'scheduled' AND cron_expression IS NOT NULL) OR type != 'scheduled'
    ),
    CONSTRAINT scripts_http_path_required CHECK (
        (type = 'http' AND http_path IS NOT NULL) OR type != 'http'
    ),
    CONSTRAINT scripts_event_types_required CHECK (
        (type = 'event' AND array_length(event_types, 1) > 0) OR type != 'event'
    )
);

CREATE INDEX idx_scripts_tenant_id ON scripts(tenant_id);
CREATE INDEX idx_scripts_tenant_status ON scripts(tenant_id, status);
CREATE INDEX idx_scripts_tenant_type_status ON scripts(tenant_id, type, status);
CREATE INDEX idx_scripts_http_path ON scripts(tenant_id, http_path) WHERE http_path IS NOT NULL;
CREATE INDEX idx_scripts_event_types ON scripts USING GIN(event_types) WHERE array_length(event_types, 1) > 0;
CREATE INDEX idx_scripts_cron_expression ON scripts(tenant_id, cron_expression) WHERE cron_expression IS NOT NULL;
CREATE INDEX idx_scripts_metadata ON scripts USING GIN(metadata);
CREATE INDEX idx_scripts_tags ON scripts USING GIN(tags);
CREATE INDEX idx_scripts_created_at ON scripts(tenant_id, created_at DESC);
CREATE INDEX idx_scripts_fulltext ON scripts USING GIN(
    to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, ''))
);

-- Create script_executions table
CREATE TABLE script_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout', 'cancelled')),
    trigger_type VARCHAR(50) NOT NULL CHECK (trigger_type IN ('cron', 'http', 'event', 'manual', 'api')),
    trigger_data JSONB DEFAULT '{}'::jsonb,
    input JSONB DEFAULT '{}'::jsonb,
    output JSONB,
    error TEXT,
    metrics JSONB DEFAULT '{
        "duration_ms": 0,
        "memory_used_bytes": 0,
        "api_call_count": 0,
        "database_query_count": 0
    }'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_executions_script_id ON script_executions(script_id);
CREATE INDEX idx_executions_tenant_id ON script_executions(tenant_id);
CREATE INDEX idx_executions_tenant_script ON script_executions(tenant_id, script_id);
CREATE INDEX idx_executions_tenant_status ON script_executions(tenant_id, status);
CREATE INDEX idx_executions_tenant_started_at ON script_executions(tenant_id, started_at DESC);
CREATE INDEX idx_executions_trigger_type ON script_executions(tenant_id, trigger_type);
CREATE INDEX idx_executions_pending ON script_executions(tenant_id, status) WHERE status = 'pending';
CREATE INDEX idx_executions_running ON script_executions(tenant_id, status) WHERE status = 'running';

-- Create script_versions table
CREATE TABLE script_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    source TEXT NOT NULL,
    change_description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT script_versions_unique UNIQUE (script_id, version_number)
);

CREATE INDEX idx_versions_script_id ON script_versions(script_id);
CREATE INDEX idx_versions_tenant_id ON script_versions(tenant_id);
CREATE INDEX idx_versions_created_at ON script_versions(script_id, created_at DESC);

-- Create script_event_subscriptions table
CREATE TABLE script_event_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT script_event_subscriptions_unique UNIQUE (script_id, event_type)
);

CREATE INDEX idx_event_subs_tenant_event ON script_event_subscriptions(tenant_id, event_type) WHERE is_active = true;
CREATE INDEX idx_event_subs_script_id ON script_event_subscriptions(script_id);

-- Create script_scheduled_jobs table
CREATE TABLE script_scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL UNIQUE REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    cron_expression VARCHAR(100) NOT NULL,
    timezone VARCHAR(100) DEFAULT 'UTC',
    next_run_at TIMESTAMPTZ NOT NULL,
    last_run_at TIMESTAMPTZ,
    last_run_status VARCHAR(50),
    is_running BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scheduled_jobs_next_run ON script_scheduled_jobs(next_run_at) WHERE is_running = false;
CREATE INDEX idx_scheduled_jobs_tenant_id ON script_scheduled_jobs(tenant_id);
CREATE INDEX idx_scheduled_jobs_script_id ON script_scheduled_jobs(script_id);

-- Create script_http_endpoints table
CREATE TABLE script_http_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL UNIQUE REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    http_path VARCHAR(500) NOT NULL,
    http_methods TEXT[] NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT script_http_endpoints_path_unique UNIQUE (tenant_id, http_path)
);

CREATE INDEX idx_http_endpoints_path ON script_http_endpoints(tenant_id, http_path) WHERE is_active = true;
CREATE INDEX idx_http_endpoints_script_id ON script_http_endpoints(script_id);

-- Create script_event_dead_letters table
CREATE TABLE script_event_dead_letters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    execution_id UUID REFERENCES script_executions(id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    error TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dead_letters_tenant_id ON script_event_dead_letters(tenant_id);
CREATE INDEX idx_dead_letters_script_id ON script_event_dead_letters(script_id);
CREATE INDEX idx_dead_letters_created_at ON script_event_dead_letters(tenant_id, created_at DESC);
CREATE INDEX idx_dead_letters_retry_count ON script_event_dead_letters(retry_count) WHERE retry_count < 3;

-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin

DROP TABLE IF EXISTS script_event_dead_letters CASCADE;
DROP TABLE IF EXISTS script_http_endpoints CASCADE;
DROP TABLE IF EXISTS script_scheduled_jobs CASCADE;
DROP TABLE IF EXISTS script_event_subscriptions CASCADE;
DROP TABLE IF EXISTS script_versions CASCADE;
DROP TABLE IF EXISTS script_executions CASCADE;
DROP TABLE IF EXISTS scripts CASCADE;

-- +migrate StatementEnd
```

## Performance Considerations

### Query Optimization

**Active script lookup (scheduler):**
```sql
-- Uses: idx_scripts_tenant_type_status
SELECT id, name, source, resource_limits, cron_expression
FROM scripts
WHERE tenant_id = $1
  AND type = 'scheduled'
  AND status = 'active';
```

**HTTP endpoint lookup:**
```sql
-- Uses: idx_http_endpoints_path
SELECT s.id, s.name, s.source, s.resource_limits
FROM scripts s
JOIN script_http_endpoints e ON e.script_id = s.id
WHERE e.tenant_id = $1
  AND e.http_path = $2
  AND e.is_active = true
  AND s.status = 'active';
```

**Event subscription lookup:**
```sql
-- Uses: idx_event_subs_tenant_event
SELECT s.id, s.name, s.source, s.resource_limits
FROM scripts s
JOIN script_event_subscriptions sub ON sub.script_id = s.id
WHERE sub.tenant_id = $1
  AND sub.event_type = $2
  AND sub.is_active = true
  AND s.status = 'active';
```

**Execution history (paginated):**
```sql
-- Uses: idx_executions_tenant_started_at
SELECT id, script_id, status, trigger_type, started_at, completed_at, error
FROM script_executions
WHERE tenant_id = $1
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;
```

### Index Strategy

**GIN Indexes** (for array/JSONB columns):
- `event_types` - Fast event type lookup
- `metadata` - JSONB key/value search
- `tags` - Tag-based filtering
- Full-text search on name/description

**Partial Indexes** (for filtered queries):
- `http_path IS NOT NULL` - Only HTTP endpoint scripts
- `status = 'pending'` - Pending executions
- `is_running = false` - Available scheduled jobs
- `retry_count < 3` - Retriable dead letters

**Composite Indexes** (for common query patterns):
- `(tenant_id, status)` - Active scripts by tenant
- `(tenant_id, type, status)` - Scripts by type and status
- `(tenant_id, script_id)` - Execution history per script

### Retention Policy

**Execution History Cleanup** (after 90 days):
```sql
DELETE FROM script_executions
WHERE started_at < NOW() - INTERVAL '90 days'
  AND status IN ('completed', 'failed', 'timeout');
```

**Dead Letter Cleanup** (after 30 days, if retry_count >= 3):
```sql
DELETE FROM script_event_dead_letters
WHERE created_at < NOW() - INTERVAL '30 days'
  AND retry_count >= 3;
```

## Acceptance Criteria

### Schema Design
- [ ] All tables include `tenant_id` with foreign key to tenants
- [ ] CHECK constraints enforce enum values (type, status, trigger_type)
- [ ] UNIQUE constraints prevent duplicate names, paths per tenant
- [ ] Foreign key CASCADE deletes for script-related tables
- [ ] JSONB columns for flexible metadata, limits, metrics
- [ ] TEXT[] arrays for http_methods, event_types, tags

### Indexes
- [ ] Tenant isolation indexes on all tables
- [ ] Composite indexes for common query patterns
- [ ] Partial indexes for filtered queries (active, pending, etc.)
- [ ] GIN indexes for array and JSONB columns
- [ ] Full-text search index for script discovery

### Migrations
- [ ] Up migration creates all tables and indexes
- [ ] Down migration drops tables in reverse dependency order
- [ ] StatementBegin/StatementEnd wrappers for complex DDL
- [ ] Migration filename includes timestamp: `{timestamp}_create_jsruntime_tables.sql`
- [ ] Test reversibility: Up → Down → Up cycle

### Multi-Tenant Isolation
- [ ] All queries include `tenant_id` in WHERE clause
- [ ] UNIQUE constraints scoped to tenant (name, http_path)
- [ ] Indexes leverage `tenant_id` for partition pruning
- [ ] No cross-tenant data access possible

### Data Integrity
- [ ] Foreign key constraints for referential integrity
- [ ] CHECK constraints for business rule validation
- [ ] NOT NULL constraints for required fields
- [ ] DEFAULT values for timestamps, arrays, JSONB
