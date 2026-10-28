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
    method varchar(255) NOT NULL,
    path varchar(255) NOT NULL,
    user_id int REFERENCES users (id) ON DELETE SET NULL,
    after JSON,
    before JSON,
    user_agent varchar(255) NOT NULL,
    ip varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE INDEX action_logs_tenant_id_idx ON action_logs (tenant_id);

CREATE INDEX action_log_user_id_idx ON action_logs (user_id);

CREATE INDEX authentication_logs_tenant_id_idx ON authentication_logs (tenant_id);

CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);

CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

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

CREATE TRIGGER business_events_append_only
    BEFORE UPDATE OR DELETE ON business_events
    FOR EACH ROW
    EXECUTE FUNCTION business_events_append_only ();


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
    FOREIGN KEY (tenant_id, event_id) REFERENCES business_events (tenant_id, event_id) ON DELETE CASCADE
);

CREATE INDEX business_event_deliveries_due_idx ON business_event_deliveries (available_at, created_at)
WHERE
    delivered_at IS NULL;
