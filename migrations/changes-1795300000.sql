-- Human tasks assigned to users, permissions, roles and groups, decided by a
-- quorum, delegated and escalated (#1327).
--
-- human_task_assignees is the authoritative list of who may decide a task:
-- the assignees of the task, those its escalation steps added, and the users
-- a delegation passed a decision to (delegated_from set). The assignee_*
-- columns of human_tasks stay for compatibility: they are written when a task
-- is created with exactly one user or permission assignee, otherwise
-- assignee_kind is ''. Every existing task gets its one assignee here, and
-- every decided task its decision.
-- +migrate Up
ALTER TABLE extruntime.human_tasks
    ADD COLUMN required integer NOT NULL DEFAULT 1 CHECK (required BETWEEN 1 AND 100),
    ADD COLUMN outcomes text[],
    ADD COLUMN veto text[],
    ADD COLUMN escalation jsonb,
    ADD COLUMN escalation_level integer NOT NULL DEFAULT 0,
    ADD COLUMN allow_delegation boolean NOT NULL DEFAULT FALSE,
    ADD COLUMN choice text NOT NULL DEFAULT '';

ALTER TABLE extruntime.human_tasks
    DROP CONSTRAINT IF EXISTS human_tasks_assignee_check;

ALTER TABLE extruntime.human_tasks
    DROP CONSTRAINT IF EXISTS human_tasks_assignee_kind_check;

ALTER TABLE extruntime.human_tasks
    ADD CONSTRAINT human_tasks_assignee_check CHECK ((assignee_kind = '' AND assignee_user_id IS NULL AND assignee_permission = '') OR (assignee_kind = 'user' AND assignee_user_id IS NOT NULL AND assignee_permission = '') OR (assignee_kind = 'permission' AND assignee_user_id IS NULL AND assignee_permission <> ''));

-- An applied escalation step is an occurrence of its own (kind
-- 'escalation'): its key makes a repeat after a crash a no-op.
ALTER TABLE extruntime.workflow_occurrences
    DROP CONSTRAINT IF EXISTS workflow_occurrences_kind_check;

ALTER TABLE extruntime.workflow_occurrences
    ADD CONSTRAINT workflow_occurrences_kind_check CHECK (kind IN ('transaction', 'step', 'sleep', 'waitFor', 'task', 'call', 'start', 'now', 'uuid', 'escalation'));

CREATE TABLE extruntime.human_task_assignees (
    id bigserial PRIMARY KEY,
    task_id uuid NOT NULL REFERENCES extruntime.human_tasks (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('user', 'permission', 'role', 'group')),
    user_id bigint,
    ref text NOT NULL DEFAULT '',
    delegated_from bigint,
    comment text NOT NULL DEFAULT '',
    added_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT human_task_assignees_shape_check CHECK ((kind = 'user' AND user_id IS NOT NULL AND ref = '') OR (kind <> 'user' AND user_id IS NULL AND ref <> '' AND delegated_from IS NULL))
);

CREATE INDEX human_task_assignees_task_idx ON extruntime.human_task_assignees (task_id);

CREATE INDEX human_task_assignees_user_idx ON extruntime.human_task_assignees (tenant_id, user_id)
WHERE
    user_id IS NOT NULL;

CREATE INDEX human_task_assignees_ref_idx ON extruntime.human_task_assignees (tenant_id, kind, ref)
WHERE
    ref <> '';

CREATE TABLE extruntime.human_task_decisions (
    task_id uuid NOT NULL REFERENCES extruntime.human_tasks (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    choice text NOT NULL DEFAULT '',
    decision jsonb,
    delegated_from bigint,
    decided_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (task_id, user_id)
);

CREATE INDEX human_task_decisions_tenant_user_idx ON extruntime.human_task_decisions (tenant_id, user_id);

INSERT INTO extruntime.human_task_assignees (task_id, tenant_id, kind, user_id, ref, added_at)
SELECT
    id,
    tenant_id,
    assignee_kind,
    CASE WHEN assignee_kind = 'user' THEN assignee_user_id END,
    CASE WHEN assignee_kind = 'permission' THEN assignee_permission ELSE '' END,
    created_at
FROM
    extruntime.human_tasks
WHERE
    assignee_kind IN ('user', 'permission');

INSERT INTO extruntime.human_task_decisions (task_id, tenant_id, user_id, decision, decided_at)
SELECT
    id,
    tenant_id,
    decided_by,
    decision,
    COALESCE(decided_at, created_at)
FROM
    extruntime.human_tasks
WHERE
    status = 'decided'
    AND decided_by IS NOT NULL;

-- +migrate Down
DROP TABLE IF EXISTS extruntime.human_task_decisions;

DROP TABLE IF EXISTS extruntime.human_task_assignees;

DELETE FROM extruntime.workflow_occurrences
WHERE kind = 'escalation';

ALTER TABLE extruntime.workflow_occurrences
    DROP CONSTRAINT IF EXISTS workflow_occurrences_kind_check;

ALTER TABLE extruntime.workflow_occurrences
    ADD CONSTRAINT workflow_occurrences_kind_check CHECK (kind IN ('transaction', 'step', 'sleep', 'waitFor', 'task', 'call', 'start', 'now', 'uuid'));

-- Tasks without a single user or permission assignee cannot be represented
-- by the previous schema.
DELETE FROM extruntime.human_tasks
WHERE assignee_kind NOT IN ('user', 'permission');

ALTER TABLE extruntime.human_tasks
    DROP CONSTRAINT IF EXISTS human_tasks_assignee_check;

ALTER TABLE extruntime.human_tasks
    ADD CONSTRAINT human_tasks_assignee_kind_check CHECK (assignee_kind IN ('user', 'permission'));

ALTER TABLE extruntime.human_tasks
    ADD CONSTRAINT human_tasks_assignee_check CHECK ((assignee_kind = 'user' AND assignee_user_id IS NOT NULL AND assignee_permission = '') OR (assignee_kind = 'permission' AND assignee_user_id IS NULL AND assignee_permission <> ''));

ALTER TABLE extruntime.human_tasks
    DROP COLUMN IF EXISTS choice,
    DROP COLUMN IF EXISTS allow_delegation,
    DROP COLUMN IF EXISTS escalation_level,
    DROP COLUMN IF EXISTS escalation,
    DROP COLUMN IF EXISTS veto,
    DROP COLUMN IF EXISTS outcomes,
    DROP COLUMN IF EXISTS required;
