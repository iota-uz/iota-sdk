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

-- One row per applied escalation step, committed with the task change: its
-- key makes a repeat after a crash a no-op. Kept apart from
-- workflow_occurrences so no workflow occurrence name can take its key.
CREATE TABLE extruntime.human_task_escalations (
    task_id uuid NOT NULL REFERENCES extruntime.human_tasks (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    step integer NOT NULL CHECK (step >= 0),
    added jsonb,
    applied_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (task_id, step)
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
-- A task still pending (open, or closed while its workflow occurrence still
-- waits for the result) that the previous schema cannot represent — no single
-- user or permission assignee, a quorum, added or delegated assignees, a
-- vote that has not closed it, or escalation steps not yet applied — would
-- lose its assignees, votes or planned steps: the downgrade is refused while
-- one exists, before anything is dropped. Finished tasks of that shape are
-- history and are removed below.
-- +migrate StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM extruntime.human_tasks t
        WHERE (t.status = 'open'
               OR EXISTS (SELECT 1 FROM extruntime.workflow_occurrences o
                          WHERE o.instance_id = t.instance_id AND o.name = t.name AND o.n = t.n AND o.status = 'waiting'))
          AND (t.assignee_kind NOT IN ('user', 'permission')
               OR t.required > 1
               OR (SELECT count(*) FROM extruntime.human_task_assignees a WHERE a.task_id = t.id) > 1
               OR (t.status = 'open' AND EXISTS (SELECT 1 FROM extruntime.human_task_decisions d WHERE d.task_id = t.id))
               OR (t.status = 'open' AND t.escalation_level < COALESCE(jsonb_array_length(t.escalation), 0))))
    THEN
        RAISE EXCEPTION 'extruntime.human_tasks: pending tasks use assignees, quorums, delegations or votes the previous schema cannot hold; decide or cancel them before downgrading';
    END IF;
END
$$;
-- +migrate StatementEnd

DROP TABLE IF EXISTS extruntime.human_task_escalations;

DROP TABLE IF EXISTS extruntime.human_task_decisions;

DROP TABLE IF EXISTS extruntime.human_task_assignees;

-- Finished tasks without a single user or permission assignee cannot be
-- represented by the previous schema (pending ones refused the downgrade).
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
