-- +migrate Up
-- Backtests of automations (#1389): datasets of events an automation owns, their labels, the jobs that
-- import history into them, and backtests (a version of the graph run over a dataset in capture mode)
-- with a result per example. A dataset's events are copies: pruning the outbox (the SDK's event journal)
-- keeps them.
CREATE TABLE automation.datasets (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid NOT NULL,
    automation_id uuid NOT NULL,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '',
    created_by bigint NOT NULL,
    created_origin text NOT NULL CHECK (created_origin IN ('human', 'agent')),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, automation_id) REFERENCES automation.automations (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX datasets_automation_idx ON automation.datasets (tenant_id, automation_id, created_at DESC);

-- An example: an event ({kind, ...payload}) and the state of its subject at the event. source_ref is the
-- source's identity of the event (the outbox event's id, a connector's message id): one event is in a
-- dataset once. snapshot NULL: the subject has no variables; snapshot_approximate: the record's state
-- was read when the event was imported, not as it was at the event.
CREATE TABLE automation.dataset_examples (
    tenant_id uuid NOT NULL,
    dataset_id uuid NOT NULL,
    id uuid NOT NULL,
    source text NOT NULL CHECK (source IN ('journal', 'history', 'synthetic')),
    source_ref text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    event_kind text NOT NULL,
    event jsonb NOT NULL,
    subject_entity text NOT NULL DEFAULT '',
    subject_key text NOT NULL DEFAULT '',
    snapshot jsonb,
    snapshot_approximate boolean NOT NULL DEFAULT FALSE,
    occurred_at timestamp with time zone NOT NULL,
    created_by bigint NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, dataset_id) REFERENCES automation.datasets (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX dataset_examples_dataset_idx ON automation.dataset_examples (tenant_id, dataset_id, occurred_at, id);

CREATE UNIQUE INDEX dataset_examples_source_idx ON automation.dataset_examples (tenant_id, dataset_id, source, source_ref)
WHERE
    source_ref <> '';

-- The expected outcome of a node for an example: expected is given with a synthetic event, manual is a
-- person's label and wins over it.
CREATE TABLE automation.dataset_labels (
    tenant_id uuid NOT NULL,
    example_id uuid NOT NULL,
    node_id text NOT NULL CHECK (length(node_id) BETWEEN 1 AND 100),
    source text NOT NULL CHECK (source IN ('manual', 'expected')),
    outcome text NOT NULL CHECK (length(outcome) BETWEEN 1 AND 100),
    set_by bigint NOT NULL,
    set_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, example_id, node_id, source),
    FOREIGN KEY (tenant_id, example_id) REFERENCES automation.dataset_examples (tenant_id, id) ON DELETE CASCADE
);

-- A job that imports events into a dataset; the worker runs it. heartbeat_at is the lease of the worker
-- that runs it: a job whose lease expired is taken again.
CREATE TABLE automation.dataset_imports (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    dataset_id uuid NOT NULL,
    source jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    imported integer NOT NULL DEFAULT 0,
    duplicates integer NOT NULL DEFAULT 0,
    overflow integer NOT NULL DEFAULT 0,
    unreadable integer NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_by bigint NOT NULL,
    created_origin text NOT NULL CHECK (created_origin IN ('human', 'agent')),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    heartbeat_at timestamp with time zone,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, dataset_id) REFERENCES automation.datasets (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX dataset_imports_dataset_idx ON automation.dataset_imports (tenant_id, dataset_id, created_at DESC);

CREATE INDEX dataset_imports_pending_idx ON automation.dataset_imports (created_at)
WHERE
    status IN ('queued', 'running');

-- A backtest: one version of a graph over a dataset. decisions is cached or live.
CREATE TABLE automation.backtests (
    tenant_id uuid NOT NULL,
    id uuid NOT NULL,
    automation_id uuid NOT NULL,
    dataset_id uuid NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    decisions text NOT NULL CHECK (decisions IN ('cached', 'live')),
    status text NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed', 'cancelled')),
    total integer NOT NULL DEFAULT 0,
    done integer NOT NULL DEFAULT 0,
    live_calls integer NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    created_by bigint NOT NULL,
    created_origin text NOT NULL CHECK (created_origin IN ('human', 'agent')),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    heartbeat_at timestamp with time zone,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, dataset_id) REFERENCES automation.datasets (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, automation_id) REFERENCES automation.automations (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX backtests_automation_idx ON automation.backtests (tenant_id, automation_id, created_at DESC);

CREATE INDEX backtests_pending_idx ON automation.backtests (created_at)
WHERE
    status IN ('queued', 'running');

-- What the graph would have done for one example of a backtest: its trace, the first outcome of each
-- node, the decisions of models and the effects it would have made.
CREATE TABLE automation.backtest_results (
    tenant_id uuid NOT NULL,
    backtest_id uuid NOT NULL,
    example_id uuid NOT NULL,
    end_kind text NOT NULL,
    outcomes jsonb NOT NULL DEFAULT '{}'::jsonb,
    decisions jsonb NOT NULL DEFAULT '[]'::jsonb,
    effects jsonb NOT NULL DEFAULT '[]'::jsonb,
    trace jsonb NOT NULL DEFAULT '[]'::jsonb,
    error text NOT NULL DEFAULT '',
    live_calls integer NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, backtest_id, example_id),
    FOREIGN KEY (tenant_id, backtest_id) REFERENCES automation.backtests (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, example_id) REFERENCES automation.dataset_examples (tenant_id, id) ON DELETE CASCADE
);

-- +migrate Down
DROP TABLE IF EXISTS automation.backtest_results;

DROP TABLE IF EXISTS automation.backtests;

DROP TABLE IF EXISTS automation.dataset_imports;

DROP TABLE IF EXISTS automation.dataset_labels;

DROP TABLE IF EXISTS automation.dataset_examples;

DROP TABLE IF EXISTS automation.datasets;
