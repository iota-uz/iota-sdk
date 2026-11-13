-- +migrate Up
-- Human tasks assigned by permission: a task is assigned to one user or to anyone holding a permission.
ALTER TABLE extruntime.human_tasks
    ADD COLUMN assignee_kind text NOT NULL DEFAULT 'user' CHECK (assignee_kind IN ('user', 'permission')),
    ADD COLUMN assignee_permission text NOT NULL DEFAULT '',
    ALTER COLUMN assignee_user_id DROP NOT NULL,
    ADD CONSTRAINT human_tasks_assignee_check CHECK ((assignee_kind = 'user' AND assignee_user_id IS NOT NULL AND assignee_permission = '') OR (assignee_kind = 'permission' AND assignee_user_id IS NULL AND assignee_permission <> ''));

CREATE INDEX human_tasks_assignee_permission_idx ON extruntime.human_tasks (tenant_id, assignee_permission, status)
WHERE
    assignee_kind = 'permission';

-- +migrate Down
DROP INDEX IF EXISTS extruntime.human_tasks_assignee_permission_idx;

ALTER TABLE extruntime.human_tasks
    DROP CONSTRAINT IF EXISTS human_tasks_assignee_check;

DELETE FROM extruntime.human_tasks
WHERE assignee_kind = 'permission';

ALTER TABLE extruntime.human_tasks
    DROP COLUMN IF EXISTS assignee_permission,
    DROP COLUMN IF EXISTS assignee_kind,
    ALTER COLUMN assignee_user_id SET NOT NULL;
