-- The revision of an automation's canvas layout (#1339): a layout save names the revision it was made
-- over, and a save over a stale one is refused (VERSION_CONFLICT), so a second tab does not overwrite
-- the positions of the first.
-- +migrate Up
ALTER TABLE automation.automations
    ADD COLUMN layout_revision integer NOT NULL DEFAULT 0;

-- +migrate Down
ALTER TABLE automation.automations
    DROP COLUMN IF EXISTS layout_revision;
