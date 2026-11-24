-- The lines a call node logged during its step (engine.Log), masked with the connection's secrets: the run
-- view's node drawer shows them (#1347). NULL: the node logged nothing.
-- +migrate Up
ALTER TABLE automation.steps
    ADD COLUMN logs jsonb;

-- +migrate Down
ALTER TABLE automation.steps
    DROP COLUMN IF EXISTS logs;
