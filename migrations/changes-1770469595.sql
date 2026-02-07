-- +migrate Up
-- Persist assistant debug trace for deterministic debug mode rendering.
ALTER TABLE bichat.messages
    ADD COLUMN IF NOT EXISTS debug_trace jsonb;

-- +migrate Down
ALTER TABLE bichat.messages
    DROP COLUMN IF EXISTS debug_trace;
