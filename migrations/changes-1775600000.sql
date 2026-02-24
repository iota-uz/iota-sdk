-- +migrate Up
-- BiChat sharing and group chat foundations.
-- Adds explicit session membership and user-message author attribution.

CREATE TABLE IF NOT EXISTS bichat.session_members (
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES bichat.sessions (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    role varchar(16) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, session_id, user_id),
    CONSTRAINT session_members_role_check CHECK (role IN ('EDITOR', 'VIEWER'))
);

CREATE INDEX IF NOT EXISTS idx_session_members_user
    ON bichat.session_members (tenant_id, user_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_members_session
    ON bichat.session_members (tenant_id, session_id, role);

ALTER TABLE IF EXISTS bichat.messages
    ADD COLUMN IF NOT EXISTS author_user_id bigint REFERENCES public.users (id) ON DELETE RESTRICT;

UPDATE bichat.messages AS m
SET author_user_id = s.user_id
FROM bichat.sessions AS s
WHERE m.session_id = s.id
  AND m.role = 'user'
  AND m.author_user_id IS NULL;

ALTER TABLE IF EXISTS bichat.messages
    DROP CONSTRAINT IF EXISTS messages_user_requires_author;

ALTER TABLE IF EXISTS bichat.messages
    ADD CONSTRAINT messages_user_requires_author
    CHECK (role <> 'user' OR author_user_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS idx_messages_author
    ON bichat.messages (author_user_id)
    WHERE author_user_id IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS bichat.idx_messages_author;

ALTER TABLE IF EXISTS bichat.messages
    DROP CONSTRAINT IF EXISTS messages_user_requires_author;

ALTER TABLE IF EXISTS bichat.messages
    DROP COLUMN IF EXISTS author_user_id;

DROP INDEX IF EXISTS bichat.idx_session_members_session;
DROP INDEX IF EXISTS bichat.idx_session_members_user;
DROP TABLE IF EXISTS bichat.session_members;
