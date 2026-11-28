-- The due time an escalation step replaced and the transaction that applied it (#1446): the inbox pages
-- by a task's due time as the snapshot of the traversal's first page saw it, so a task an escalation moved
-- later (committed after that snapshot, even if it began before) is not listed again on a later page.
-- NULL: a step applied before these columns existed (counted as seen).
-- +migrate Up
ALTER TABLE extruntime.human_task_escalations
    ADD COLUMN due_before timestamp with time zone,
    ADD COLUMN applied_xact xid8;

ALTER TABLE extruntime.human_task_escalations
    ALTER COLUMN applied_xact SET DEFAULT pg_current_xact_id();

-- +migrate Down
ALTER TABLE extruntime.human_task_escalations
    DROP COLUMN IF EXISTS applied_xact,
    DROP COLUMN IF EXISTS due_before;
