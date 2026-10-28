-- +migrate Up
CREATE TABLE business_events (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    event_type varchar(200) NOT NULL,
    schema_version integer NOT NULL CHECK (schema_version > 0),
    category varchar(20) NOT NULL CHECK (category IN ('business', 'security', 'legacy')),
    occurred_at timestamp with time zone,
    recorded_at timestamp with time zone NOT NULL DEFAULT now(),
    actor_kind varchar(20) CHECK (actor_kind IN ('user', 'service', 'system')),
    actor_id varchar(255),
    actor_snapshot jsonb,
    initiator_snapshot jsonb,
    channel varchar(20) CHECK (channel IN ('ui', 'mcp', 'chat', 'import', 'job', 'api')),
    operation_id varchar(255),
    request_id varchar(255),
    run_id varchar(255),
    client_id varchar(255),
    outcome varchar(20) CHECK (outcome IN ('succeeded', 'failed', 'denied')),
    resources text[] NOT NULL DEFAULT '{}',
    subject_refs text[] NOT NULL DEFAULT '{}',
    subjects jsonb NOT NULL DEFAULT '[]',
    changes jsonb NOT NULL DEFAULT '[]',
    fingerprint varchar(64),
    legacy jsonb,
    PRIMARY KEY (tenant_id, event_id),
    CONSTRAINT business_events_complete CHECK (legacy IS NOT NULL OR (occurred_at IS NOT NULL AND actor_kind IS NOT NULL AND actor_id IS NOT NULL AND actor_snapshot IS NOT NULL AND channel IS NOT NULL AND operation_id IS NOT NULL AND outcome IS NOT NULL AND fingerprint IS NOT NULL))
);

CREATE INDEX business_events_timeline_idx ON business_events (tenant_id, occurred_at DESC, event_id DESC);

CREATE INDEX business_events_operation_idx ON business_events (tenant_id, operation_id);

CREATE INDEX business_events_actor_idx ON business_events (tenant_id, actor_kind, actor_id, occurred_at DESC);

CREATE INDEX business_events_type_idx ON business_events (tenant_id, event_type, occurred_at DESC);

CREATE INDEX business_events_resources_idx ON business_events USING gin (resources);

CREATE INDEX business_events_subject_refs_idx ON business_events USING gin (subject_refs);

-- +migrate StatementBegin
CREATE FUNCTION business_events_append_only ()
    RETURNS TRIGGER
    AS $$
BEGIN
    IF TG_OP = 'DELETE' AND NOT EXISTS (
        SELECT
            1
        FROM
            tenants
        WHERE
            id = OLD.tenant_id) THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'business_events is append-only';
END;
$$
LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TRIGGER business_events_append_only
    BEFORE UPDATE OR DELETE ON business_events
    FOR EACH ROW
    EXECUTE FUNCTION business_events_append_only ();

INSERT INTO business_events (tenant_id, event_id, event_type, schema_version, category, occurred_at, recorded_at, actor_kind, actor_id, legacy)
SELECT
    tenant_id,
    md5('action_logs:' || id)::uuid,
    'logging.action_log',
    1,
    'legacy',
    created_at,
    COALESCE(created_at, now()),
    CASE WHEN user_id IS NOT NULL THEN
        'user'
    END,
    user_id::text,
    jsonb_build_object('source', 'action_logs', 'source_id', id::text, 'incomplete', TRUE, 'payload', jsonb_build_object('method', method, 'path', path, 'before', before::jsonb, 'after', after::jsonb, 'user_agent', user_agent, 'ip', ip)) || CASE WHEN user_id IS NOT NULL THEN
        jsonb_build_object('user_id', user_id::text)
    ELSE
        '{}'::jsonb
    END
FROM
    action_logs
ON CONFLICT (tenant_id,
    event_id)
    DO NOTHING;

COMMENT ON TABLE action_logs IS 'Superseded by business_events; existing rows are copied there as legacy events.';

-- +migrate Down
COMMENT ON TABLE action_logs IS NULL;

DROP TABLE IF EXISTS business_events;

DROP FUNCTION IF EXISTS business_events_append_only ();
