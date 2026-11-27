-- The lines a call node logged during its step (engine.Log), masked with the connection's secrets: the run
-- view's node drawer shows them (#1347). NULL: the node logged nothing.
-- +migrate Up
ALTER TABLE automation.steps
    ADD COLUMN logs jsonb;

-- A node test an author confirmed, by its key (#1347): the first request claims the key and runs the
-- test; a repeat on any process gets the stored answer, and while there is none (the test runs, or its
-- process died mid-call) it is refused rather than run again. result NULL: no answer recorded (or it
-- expired: answers are kept a day). A claim is never deleted, so a key once used never runs a test again.
CREATE TABLE automation.node_tests (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    key text NOT NULL,
    node_id text NOT NULL,
    result jsonb,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, key)
);

CREATE INDEX node_tests_created_idx ON automation.node_tests (tenant_id, created_at);

-- +migrate Down
DROP TABLE IF EXISTS automation.node_tests;

ALTER TABLE automation.steps
    DROP COLUMN IF EXISTS logs;
