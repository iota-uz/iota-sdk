-- Obligations: cancellation, the account an obligation reserves money on, and an optional project.
-- +migrate Up
ALTER TABLE debts DROP CONSTRAINT IF EXISTS debts_status_check;

ALTER TABLE debts
    ADD CONSTRAINT debts_status_check CHECK (status IN ('PENDING', 'SETTLED', 'PARTIAL', 'WRITTEN_OFF', 'CANCELLED'));

ALTER TABLE debts
    ADD COLUMN money_account_id uuid REFERENCES money_accounts (id) ON DELETE SET NULL,
    ADD COLUMN project_id uuid REFERENCES projects (id) ON DELETE SET NULL;

CREATE INDEX debts_money_account_id_idx ON debts (money_account_id);

CREATE INDEX debts_project_id_idx ON debts (project_id);

-- +migrate Down
DROP INDEX IF EXISTS debts_project_id_idx;

DROP INDEX IF EXISTS debts_money_account_id_idx;

ALTER TABLE debts
    DROP COLUMN IF EXISTS project_id,
    DROP COLUMN IF EXISTS money_account_id;

UPDATE debts SET status = 'WRITTEN_OFF' WHERE status = 'CANCELLED';

ALTER TABLE debts DROP CONSTRAINT IF EXISTS debts_status_check;

ALTER TABLE debts
    ADD CONSTRAINT debts_status_check CHECK (status IN ('PENDING', 'SETTLED', 'PARTIAL', 'WRITTEN_OFF'));
