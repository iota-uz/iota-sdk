-- +migrate Up
ALTER TABLE action_logs
    ALTER COLUMN method DROP NOT NULL,
    ALTER COLUMN path DROP NOT NULL,
    ALTER COLUMN user_agent DROP NOT NULL,
    ALTER COLUMN ip DROP NOT NULL,
    ADD COLUMN event_id uuid,
    ADD COLUMN event_type varchar(200),
    ADD COLUMN schema_version integer CHECK (schema_version > 0),
    ADD COLUMN category varchar(20) CHECK (category IN ('business', 'security')),
    ADD COLUMN occurred_at timestamp with time zone,
    ADD COLUMN actor_kind varchar(20) CHECK (actor_kind IN ('user', 'service', 'system')),
    ADD COLUMN actor_id varchar(255),
    ADD COLUMN actor_snapshot jsonb,
    ADD COLUMN initiator_snapshot jsonb,
    ADD COLUMN channel varchar(20) CHECK (channel IN ('ui', 'mcp', 'chat', 'import', 'job', 'api')),
    ADD COLUMN operation_id varchar(255),
    ADD COLUMN request_id varchar(255),
    ADD COLUMN run_id varchar(255),
    ADD COLUMN client_id varchar(255),
    ADD COLUMN outcome varchar(20) CHECK (outcome IN ('succeeded', 'failed', 'denied')),
    ADD COLUMN resources text[] NOT NULL DEFAULT '{}',
    ADD COLUMN subject_refs text[] NOT NULL DEFAULT '{}',
    ADD COLUMN subjects jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN changes jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN fingerprint varchar(64),
    ADD CONSTRAINT action_logs_legacy_request CHECK (event_type IS NOT NULL OR (method IS NOT NULL AND path IS NOT NULL AND user_agent IS NOT NULL AND ip IS NOT NULL)),
    ADD CONSTRAINT action_logs_event_complete CHECK (event_type IS NULL OR (schema_version IS NOT NULL AND category IS NOT NULL AND occurred_at IS NOT NULL AND created_at IS NOT NULL AND actor_kind IS NOT NULL AND actor_id IS NOT NULL AND actor_snapshot IS NOT NULL AND channel IS NOT NULL AND operation_id IS NOT NULL AND outcome IS NOT NULL AND fingerprint IS NOT NULL));

UPDATE
    action_logs
SET
    event_id = gen_random_uuid ();

ALTER TABLE action_logs
    ALTER COLUMN event_id SET DEFAULT gen_random_uuid (),
    ALTER COLUMN event_id SET NOT NULL,
    ADD CONSTRAINT action_logs_tenant_event_key UNIQUE (tenant_id, event_id);

CREATE INDEX action_logs_timeline_idx ON action_logs (tenant_id, COALESCE(occurred_at, created_at) DESC, event_id DESC);

CREATE INDEX action_logs_operation_idx ON action_logs (tenant_id, operation_id);

CREATE INDEX action_logs_actor_idx ON action_logs (tenant_id, actor_kind, actor_id, occurred_at DESC);

CREATE INDEX action_logs_event_type_idx ON action_logs (tenant_id, event_type, occurred_at DESC);

CREATE INDEX action_logs_resources_idx ON action_logs USING gin (resources);

CREATE INDEX action_logs_subject_refs_idx ON action_logs USING gin (subject_refs);

-- +migrate StatementBegin
CREATE FUNCTION action_logs_events_append_only ()
    RETURNS TRIGGER
    AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.event_type IS NULL OR NOT EXISTS (
            SELECT
                1
            FROM
                tenants
            WHERE
                id = OLD.tenant_id) THEN
            RETURN OLD;
        END IF;
    ELSIF OLD.event_type IS NULL
            AND NEW.event_type IS NULL THEN
            RETURN NEW;
    END IF;
    RAISE EXCEPTION 'action_logs events are append-only';
END;
$$
LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TRIGGER action_logs_events_append_only
    BEFORE UPDATE OR DELETE ON action_logs
    FOR EACH ROW
    EXECUTE FUNCTION action_logs_events_append_only ();

-- +migrate StatementBegin
DO $$
BEGIN
    IF to_regclass('analytics.action_logs') IS NOT NULL THEN
        DROP VIEW analytics.action_logs;
        CREATE VIEW analytics.action_logs AS
        SELECT
            id,
            method,
            path,
            user_id,
            after,
            before,
            user_agent,
            ip,
            created_at,
            tenant_id
        FROM
            public.action_logs
        WHERE
            tenant_id = current_setting('app.tenant_id', TRUE)::uuid
            AND event_type IS NULL;
    END IF;
END;
$$;
-- +migrate StatementEnd

COMMENT ON TABLE action_logs IS 'Business action history. Rows without event_type are legacy request-log records.';

-- +migrate Down
-- +migrate StatementBegin
DO $$
BEGIN
    IF to_regclass('analytics.action_logs') IS NOT NULL THEN
        DROP VIEW analytics.action_logs;
        CREATE VIEW analytics.action_logs AS
        SELECT
            id,
            method,
            path,
            user_id,
            after,
            before,
            user_agent,
            ip,
            created_at,
            tenant_id
        FROM
            public.action_logs
        WHERE
            tenant_id = current_setting('app.tenant_id', TRUE)::uuid;
    END IF;
END;
$$;
-- +migrate StatementEnd

DROP TRIGGER IF EXISTS action_logs_events_append_only ON action_logs;

DROP FUNCTION IF EXISTS action_logs_events_append_only ();

UPDATE
    action_logs
SET
    method = 'EVENT',
    path = event_type,
    user_agent = COALESCE(user_agent, ''),
    ip = COALESCE(ip, ''),
    after = to_jsonb(action_logs.*) - ARRAY['id', 'tenant_id', 'method', 'path', 'user_id', 'after', 'before', 'user_agent', 'ip', 'created_at']
WHERE
    event_type IS NOT NULL;

COMMENT ON TABLE action_logs IS NULL;

ALTER TABLE action_logs
    DROP CONSTRAINT IF EXISTS action_logs_event_complete,
    DROP CONSTRAINT IF EXISTS action_logs_legacy_request,
    DROP CONSTRAINT IF EXISTS action_logs_tenant_event_key,
    DROP COLUMN IF EXISTS event_id,
    DROP COLUMN IF EXISTS event_type,
    DROP COLUMN IF EXISTS schema_version,
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS occurred_at,
    DROP COLUMN IF EXISTS actor_kind,
    DROP COLUMN IF EXISTS actor_id,
    DROP COLUMN IF EXISTS actor_snapshot,
    DROP COLUMN IF EXISTS initiator_snapshot,
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS operation_id,
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS run_id,
    DROP COLUMN IF EXISTS client_id,
    DROP COLUMN IF EXISTS outcome,
    DROP COLUMN IF EXISTS resources,
    DROP COLUMN IF EXISTS subject_refs,
    DROP COLUMN IF EXISTS subjects,
    DROP COLUMN IF EXISTS changes,
    DROP COLUMN IF EXISTS fingerprint,
    ALTER COLUMN method SET NOT NULL,
    ALTER COLUMN path SET NOT NULL,
    ALTER COLUMN user_agent SET NOT NULL,
    ALTER COLUMN ip SET NOT NULL;
