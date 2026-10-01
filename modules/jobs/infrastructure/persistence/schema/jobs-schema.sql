CREATE TABLE jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind varchar(128) NOT NULL,
    params jsonb NOT NULL DEFAULT '{}'::jsonb,
    status varchar(16) NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    progress int NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    phase varchar(255) NOT NULL DEFAULT '',
    attempt int NOT NULL DEFAULT 0,
    result_upload_id int REFERENCES uploads (id) ON DELETE SET NULL,
    error text,
    created_at timestamp with time zone DEFAULT now (),
    updated_at timestamp with time zone DEFAULT now (),
    started_at timestamp with time zone,
    finished_at timestamp with time zone
);

CREATE INDEX jobs_tenant_user_created_idx ON jobs (tenant_id, user_id, created_at DESC);

CREATE INDEX jobs_claim_idx ON jobs (status, created_at) WHERE status = 'queued';

CREATE INDEX jobs_finished_idx ON jobs (finished_at) WHERE status IN ('done', 'failed');
