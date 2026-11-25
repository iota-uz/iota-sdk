-- The due time an escalation step replaced (#1446): the inbox pages by a task's due time as of the start
-- of the traversal, so a task an escalation moved later after a page held it is not listed again on a
-- later page. NULL: a step applied before this column existed.
-- +migrate Up
ALTER TABLE extruntime.human_task_escalations
    ADD COLUMN due_before timestamp with time zone;

-- +migrate Down
ALTER TABLE extruntime.human_task_escalations
    DROP COLUMN IF EXISTS due_before;
