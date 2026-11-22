-- +migrate Up
-- Automation engine (#1335): graph versions, enabled triggers and the durable state of runs of the
-- native workflow contract automation.run. The status of a run is the status of its workflow
-- instance; a run without an instance keeps why in start_state.
CREATE SCHEMA IF NOT EXISTS automation;

CREATE TABLE automation.automations (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    name text NOT NULL,
    latest_version integer NOT NULL CHECK (latest_version >= 1),
    enabled_version integer,
    owner_user_id bigint,
    layout jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by bigint NOT NULL,
    created_origin text NOT NULL CHECK (created_origin IN ('human', 'agent')),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone,
    PRIMARY KEY (tenant_id, id),
    CHECK (enabled_version IS NULL OR enabled_version <= latest_version)
);

CREATE INDEX automations_tenant_updated_idx ON automation.automations (tenant_id, updated_at DESC, id)
WHERE
    deleted_at IS NULL;

CREATE TABLE automation.versions (
    tenant_id uuid NOT NULL,
    automation_id uuid NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    graph jsonb NOT NULL,
    catalog_revision text NOT NULL DEFAULT '',
    code_digest text NOT NULL DEFAULT '',
    run_as text NOT NULL DEFAULT 'owner' CHECK (run_as IN ('owner', 'initiator')),
    saved_by bigint NOT NULL,
    saved_origin text NOT NULL CHECK (saved_origin IN ('human', 'agent')),
    note text NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, automation_id, version),
    FOREIGN KEY (tenant_id, automation_id) REFERENCES automation.automations (tenant_id, id) ON DELETE CASCADE
);

-- Triggers of enabled versions only: rebuilt by Enable, removed by Disable.
CREATE TABLE automation.triggers (
    tenant_id uuid NOT NULL,
    automation_id uuid NOT NULL,
    version integer NOT NULL,
    trigger_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('event', 'schedule', 'manual', 'inbound')),
    event_kind text,
    filter_cel text,
    schedule jsonb,
    -- the outbox watermark when the version was enabled: older events start nothing
    after_seq bigint NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, automation_id, trigger_id),
    FOREIGN KEY (tenant_id, automation_id, version) REFERENCES automation.versions (tenant_id, automation_id, version) ON DELETE CASCADE
);

CREATE INDEX triggers_tenant_event_idx ON automation.triggers (tenant_id, event_kind);

CREATE INDEX triggers_event_idx ON automation.triggers (event_kind);

CREATE TABLE automation.runs (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    automation_id uuid NOT NULL,
    version integer NOT NULL,
    instance_id uuid REFERENCES extruntime.workflow_instances (id) ON DELETE SET NULL,
    start_key text,
    start_state text CHECK (start_state IN ('queued', 'filter_failed', 'blocked', 'cancelled')),
    start_reason text,
    trigger_id text NOT NULL,
    trigger_event_id text,
    trigger_kind text NOT NULL CHECK (trigger_kind IN ('event', 'schedule', 'manual', 'inbound')),
    -- the trigger's event as the graph reads it ({kind, ...payload}), projected for the run-as
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    subject_entity text NOT NULL DEFAULT 'none',
    subject_key text NOT NULL DEFAULT '',
    run_as_user_id bigint,
    run_as_kind text CHECK (run_as_kind IN ('owner', 'initiator')),
    initiator_user_id bigint,
    -- the next inbox signal the run consumes, the next fenced transaction it writes
    inbox_seq bigint NOT NULL DEFAULT 0,
    tx_seq bigint NOT NULL DEFAULT 0,
    steps_count integer NOT NULL DEFAULT 0,
    depth integer NOT NULL DEFAULT 0,
    cause_run_id uuid,
    restarted_from uuid,
    output jsonb,
    error jsonb,
    -- Disable or Delete of the graph asked to cancel the run: kept until its instance is terminated,
    -- across a process failure and a later Enable
    cancel_requested boolean NOT NULL DEFAULT false,
    -- the graph's owner was told the run failed (once, whatever retried the transition)
    owner_notified_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, start_key),
    FOREIGN KEY (tenant_id, automation_id, version) REFERENCES automation.versions (tenant_id, automation_id, version) ON DELETE CASCADE,
    CHECK ((instance_id IS NULL) OR (start_state IS NULL))
);

CREATE UNIQUE INDEX runs_instance_idx ON automation.runs (instance_id)
WHERE
    instance_id IS NOT NULL;

CREATE INDEX runs_tenant_automation_created_idx ON automation.runs (tenant_id, automation_id, created_at DESC, id);

CREATE INDEX runs_tenant_created_idx ON automation.runs (tenant_id, created_at DESC, id);

CREATE INDEX runs_tenant_subject_idx ON automation.runs (tenant_id, subject_entity, subject_key);

CREATE INDEX runs_cancel_requested_idx ON automation.runs (tenant_id, id)
WHERE
    cancel_requested;

CREATE TABLE automation.tokens (
    tenant_id uuid NOT NULL,
    run_id uuid NOT NULL,
    id uuid NOT NULL,
    parent_token_id uuid,
    -- the token's name in the graph's token paths: "" the main path, "split.legal/each[3]"; a loop that
    -- forks again reuses the names of finished tokens
    path text NOT NULL,
    node_id text NOT NULL,
    from_node text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('running', 'waiting', 'joined', 'done', 'cancelled', 'failed')),
    step_seq integer NOT NULL DEFAULT 0,
    branch text,
    foreach_ref text,
    foreach_index integer,
    -- foreach.<foreach_ref>.item of an item's token
    item jsonb,
    -- node outputs the token set since it was created, in the order first set: [{node, state}]
    writes jsonb NOT NULL DEFAULT '[]'::jsonb,
    -- the state of the parallel or foreach the token waits for (it forked), and a foreach's items
    flow jsonb,
    items jsonb,
    -- how an item or a branch ended: {ok, value, error}
    arrival jsonb,
    created_seq bigint NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, run_id, id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES automation.runs (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX tokens_run_status_idx ON automation.tokens (tenant_id, run_id, status);

CREATE TABLE automation.steps (
    tenant_id uuid NOT NULL,
    run_id uuid NOT NULL,
    token_id uuid NOT NULL,
    seq integer NOT NULL,
    node_id text NOT NULL,
    attempt integer NOT NULL DEFAULT 1,
    behavior text NOT NULL DEFAULT '',
    input jsonb,
    outcome text,
    output jsonb,
    error jsonb,
    started_at timestamp with time zone NOT NULL DEFAULT now(),
    ended_at timestamp with time zone,
    PRIMARY KEY (tenant_id, run_id, token_id, seq),
    FOREIGN KEY (tenant_id, run_id) REFERENCES automation.runs (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX steps_started_idx ON automation.steps (started_at);

CREATE TABLE automation.waits (
    tenant_id uuid NOT NULL,
    run_id uuid NOT NULL,
    token_id uuid NOT NULL,
    wait_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('event', 'timer', 'join', 'task', 'call')),
    node_id text NOT NULL,
    subject_entity text,
    subject_key text,
    event_kind text,
    filter_cel text,
    timeout_at timestamp with time zone,
    status text NOT NULL CHECK (status IN ('open', 'fired', 'timed_out', 'cancelled')),
    -- the outbox watermark when the wait opened: older events do not wake it
    after_seq bigint NOT NULL DEFAULT 0,
    -- a timer occurrence, a task occurrence or a child instance is still to be cancelled
    release_pending boolean NOT NULL DEFAULT false,
    spec jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, run_id, wait_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES automation.runs (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX waits_open_event_idx ON automation.waits (tenant_id, event_kind, subject_entity, subject_key)
WHERE
    status = 'open';

CREATE INDEX waits_open_kind_idx ON automation.waits (event_kind)
WHERE
    status = 'open';

CREATE INDEX waits_run_idx ON automation.waits (tenant_id, run_id, status);

-- Runs of a conversation subject that wait for the subject's active run to finish.
CREATE TABLE automation.subject_queue (
    tenant_id uuid NOT NULL,
    subject_entity text NOT NULL,
    subject_key text NOT NULL,
    run_id uuid NOT NULL,
    seq bigserial NOT NULL,
    PRIMARY KEY (tenant_id, run_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES automation.runs (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX subject_queue_subject_idx ON automation.subject_queue (tenant_id, subject_entity, subject_key, seq);

-- The cause of an event published inside a step: {run_id, depth}. A step sets
-- app.automation_cause locally, so every event its transaction publishes carries it.
ALTER TABLE extruntime.outbox_events
    ADD COLUMN cause jsonb DEFAULT NULLIF(current_setting('app.automation_cause', TRUE), '')::jsonb;

-- +migrate Down
ALTER TABLE extruntime.outbox_events
    DROP COLUMN IF EXISTS cause;

DROP TABLE IF EXISTS automation.subject_queue;

DROP TABLE IF EXISTS automation.waits;

DROP TABLE IF EXISTS automation.steps;

DROP TABLE IF EXISTS automation.tokens;

DROP TABLE IF EXISTS automation.runs;

DROP TABLE IF EXISTS automation.triggers;

DROP TABLE IF EXISTS automation.versions;

DROP TABLE IF EXISTS automation.automations;
