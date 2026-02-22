-- Create bichat.generation_runs (required for StreamController / PostgresChatRepository.CreateRun).
-- Prerequisites: schema bichat, tables bichat.sessions, public.tenants, public.users must exist.
CREATE SCHEMA IF NOT EXISTS bichat;

CREATE TABLE IF NOT EXISTS bichat.generation_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES bichat.sessions (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    status varchar(20) NOT NULL DEFAULT 'streaming',
    partial_content text NOT NULL DEFAULT '',
    partial_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    started_at timestamptz NOT NULL DEFAULT NOW(),
    last_updated_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT generation_runs_status_check CHECK (status IN ('streaming', 'completed', 'cancelled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_generation_runs_session_active ON bichat.generation_runs (session_id)
WHERE status = 'streaming';

CREATE INDEX IF NOT EXISTS idx_generation_runs_tenant_session ON bichat.generation_runs (tenant_id, session_id);

COMMENT ON TABLE bichat.generation_runs IS 'Active streaming run state for refresh-safe resume; one row per session when streaming.';
