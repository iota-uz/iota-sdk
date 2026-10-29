CREATE TABLE authentication_logs (
    id serial PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    ip varchar(255) NOT NULL,
    user_agent varchar(255) NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE TABLE action_logs (
    id serial PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    method varchar(255),
    path varchar(255),
    user_id int REFERENCES users (id) ON DELETE SET NULL,
    after JSON,
    before JSON,
    user_agent varchar(255),
    ip varchar(255),
    created_at timestamp with time zone DEFAULT now(),
    event_id uuid NOT NULL DEFAULT gen_random_uuid (),
    event_type varchar(200),
    schema_version integer CHECK (schema_version > 0),
    category varchar(20) CHECK (category IN ('business', 'security')),
    occurred_at timestamp with time zone,
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
    CONSTRAINT action_logs_tenant_event_key UNIQUE (tenant_id, event_id),
    CONSTRAINT action_logs_legacy_request CHECK (event_type IS NOT NULL OR (method IS NOT NULL AND path IS NOT NULL AND user_agent IS NOT NULL AND ip IS NOT NULL)),
    CONSTRAINT action_logs_event_complete CHECK (event_type IS NULL OR (schema_version IS NOT NULL AND category IS NOT NULL AND occurred_at IS NOT NULL AND created_at IS NOT NULL AND actor_kind IS NOT NULL AND actor_id IS NOT NULL AND actor_snapshot IS NOT NULL AND channel IS NOT NULL AND operation_id IS NOT NULL AND outcome IS NOT NULL AND fingerprint IS NOT NULL))
);

CREATE INDEX action_logs_tenant_id_idx ON action_logs (tenant_id);

CREATE INDEX action_log_user_id_idx ON action_logs (user_id);

CREATE INDEX authentication_logs_tenant_id_idx ON authentication_logs (tenant_id);

CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);

CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

CREATE INDEX action_logs_timeline_idx ON action_logs (tenant_id, COALESCE(occurred_at, created_at) DESC, event_id DESC);

CREATE INDEX action_logs_operation_idx ON action_logs (tenant_id, operation_id);

CREATE INDEX action_logs_actor_idx ON action_logs (tenant_id, actor_kind, actor_id, occurred_at DESC);

CREATE INDEX action_logs_event_type_idx ON action_logs (tenant_id, event_type, occurred_at DESC);

CREATE INDEX action_logs_resources_idx ON action_logs USING gin (resources);

CREATE INDEX action_logs_subject_refs_idx ON action_logs USING gin (subject_refs);

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

CREATE TRIGGER action_logs_events_append_only
    BEFORE UPDATE OR DELETE ON action_logs
    FOR EACH ROW
    EXECUTE FUNCTION action_logs_events_append_only ();

CREATE TABLE business_event_deliveries (
    tenant_id uuid NOT NULL,
    event_id uuid NOT NULL,
    consumer varchar(200) NOT NULL,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    available_at timestamp with time zone NOT NULL DEFAULT now(),
    delivered_at timestamp with time zone,
    PRIMARY KEY (tenant_id, event_id, consumer),
    FOREIGN KEY (tenant_id, event_id) REFERENCES action_logs (tenant_id, event_id) ON DELETE CASCADE
);

CREATE INDEX business_event_deliveries_due_idx ON business_event_deliveries (available_at, created_at)
WHERE
    delivered_at IS NULL;

