-- +migrate Up
-- Automation layout revision (#1339, #1340): every write of a graph's layout increments it, and a save
-- of positions names the revision it was based on, so a stale editor tab or agent never overwrites
-- positions it has not seen.
ALTER TABLE automation.automations
    ADD COLUMN layout_revision integer NOT NULL DEFAULT 1;

-- +migrate Down
ALTER TABLE automation.automations
    DROP COLUMN IF EXISTS layout_revision;
