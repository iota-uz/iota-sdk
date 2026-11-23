-- +migrate Up
-- Automation engine (#1336): external calls, correlated waits for answers, inbound events of
-- connectors, merged inbound windows and the dry run's decision cache.

-- Inbound events already published, by the provider's id: a repeated delivery publishes nothing.
-- Pruned by Retention.Steps.
CREATE TABLE automation.inbound_dedup (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    connection_id text NOT NULL,
    event text NOT NULL,
    dedup_key text NOT NULL,
    seen_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, connection_id, event, dedup_key)
);

CREATE INDEX inbound_dedup_seen_idx ON automation.inbound_dedup (seen_at);

-- Open correlation tokens of ask waits. Globally unique: an inbound update of a platform credential
-- finds its tenant by the token. Deleted when the wait closes (or its first answer arrives, single).
CREATE TABLE automation.correlations (
    token text PRIMARY KEY,
    tenant_id uuid NOT NULL,
    run_id uuid NOT NULL,
    wait_id text NOT NULL,
    single boolean NOT NULL DEFAULT TRUE,
    -- the answer event the wait listens for, once it opened; only it takes the token
    event text,
    -- the first event with the token before the wait opened (its answer, if the keys match)
    answered_event text,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, run_id) REFERENCES automation.runs (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX correlations_wait_idx ON automation.correlations (tenant_id, run_id, wait_id);

CREATE INDEX correlations_expires_idx ON automation.correlations (expires_at);

-- An ask wait listens for the inbound events of its token only.
ALTER TABLE automation.waits
    ADD COLUMN correlation text;

CREATE INDEX waits_open_correlation_idx ON automation.waits (correlation)
WHERE
    status = 'open' AND correlation IS NOT NULL;

-- Warnings of a step (an assignee list cut to its limit), shown with the step.
ALTER TABLE automation.steps
    ADD COLUMN warnings jsonb;

-- The events a trigger's collect merged into the run, in order; NULL: the one event of input.
ALTER TABLE automation.runs
    ADD COLUMN events jsonb;

-- Open collect windows: inbound events of one key that start one run when the window closes.
CREATE TABLE automation.inbound_collect (
    tenant_id uuid NOT NULL,
    automation_id uuid NOT NULL,
    version integer NOT NULL,
    trigger_id text NOT NULL,
    collect_key text NOT NULL,
    -- [{id, seq, payload, occurred_at, initiator_user_id, subject}] in order
    events jsonb NOT NULL DEFAULT '[]'::jsonb,
    count integer NOT NULL DEFAULT 0,
    first_at timestamp with time zone NOT NULL,
    deadline timestamp with time zone NOT NULL,
    PRIMARY KEY (tenant_id, automation_id, trigger_id, collect_key),
    FOREIGN KEY (tenant_id, automation_id, version) REFERENCES automation.versions (tenant_id, automation_id, version) ON DELETE CASCADE
);

CREATE INDEX inbound_collect_deadline_idx ON automation.inbound_collect (deadline);

-- Call slots of a tenant (CallLimits.PerTenant): a call instance holds one while an attempt runs. A
-- slot of an instance that no longer runs (finished, or its lease expired) is dropped by the next taker.
CREATE TABLE automation.call_slots (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    instance_id uuid NOT NULL,
    acquired_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, instance_id)
);

-- The deadline an operator's Retry gave a blocked call (its first one may have passed).
CREATE TABLE automation.call_deadlines (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    instance_id uuid NOT NULL,
    deadline timestamp with time zone NOT NULL,
    PRIMARY KEY (tenant_id, instance_id)
);

-- Decisions a dry run took from a model (cached mode): sha256 of prompt, input, model and its params.
CREATE TABLE automation.decision_cache (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    key text NOT NULL,
    output jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

CREATE INDEX decision_cache_created_idx ON automation.decision_cache (created_at);

-- +migrate Down
DROP TABLE IF EXISTS automation.call_deadlines;

DROP TABLE IF EXISTS automation.call_slots;

DROP TABLE IF EXISTS automation.decision_cache;

DROP TABLE IF EXISTS automation.inbound_collect;

ALTER TABLE automation.runs
    DROP COLUMN IF EXISTS events;

ALTER TABLE automation.steps
    DROP COLUMN IF EXISTS warnings;

DROP INDEX IF EXISTS automation.waits_open_correlation_idx;

ALTER TABLE automation.waits
    DROP COLUMN IF EXISTS correlation;

DROP TABLE IF EXISTS automation.correlations;

DROP TABLE IF EXISTS automation.inbound_dedup;
