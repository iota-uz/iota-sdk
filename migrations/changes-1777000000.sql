-- +migrate Up
-- dbctl run/audit tables
CREATE TABLE IF NOT EXISTS dbctl_runs (
  id UUID PRIMARY KEY,
  operation TEXT NOT NULL,
  mode TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ,
  actor TEXT NOT NULL,
  status TEXT NOT NULL,
  target_fingerprint TEXT NOT NULL,
  policy_hash TEXT NOT NULL,
  error TEXT
);

CREATE TABLE IF NOT EXISTS dbctl_run_steps (
  run_id UUID NOT NULL REFERENCES dbctl_runs(id) ON DELETE CASCADE,
  step_id TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ,
  error TEXT,
  PRIMARY KEY (run_id, step_id)
);

CREATE TABLE IF NOT EXISTS dbctl_run_artifacts (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES dbctl_runs(id) ON DELETE CASCADE,
  artifact_type TEXT NOT NULL,
  payload_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +migrate Down
DROP TABLE IF EXISTS dbctl_run_artifacts;
DROP TABLE IF EXISTS dbctl_run_steps;
DROP TABLE IF EXISTS dbctl_runs;
