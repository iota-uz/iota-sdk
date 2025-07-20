-- +migrate Up
-- Create debts table for debt management functionality

CREATE TABLE debts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    type varchar(20) NOT NULL CHECK (type IN ('RECEIVABLE', 'PAYABLE')),
    status varchar(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SETTLED', 'PARTIAL', 'WRITTEN_OFF')),
    counterparty_id uuid NOT NULL REFERENCES counterparty (id) ON DELETE RESTRICT,
    original_amount bigint NOT NULL,
    original_amount_currency_id varchar(3) NOT NULL REFERENCES currencies (code) ON DELETE CASCADE,
    outstanding_amount bigint NOT NULL,
    outstanding_currency_id varchar(3) NOT NULL REFERENCES currencies (code) ON DELETE CASCADE,
    description text NOT NULL,
    due_date date,
    settlement_transaction_id uuid REFERENCES transactions (id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

-- Indexes for performance
CREATE INDEX debts_tenant_id_idx ON debts (tenant_id);
CREATE INDEX debts_counterparty_id_idx ON debts (counterparty_id);
CREATE INDEX debts_type_idx ON debts (type);
CREATE INDEX debts_status_idx ON debts (status);
CREATE INDEX debts_due_date_idx ON debts (due_date);
CREATE INDEX debts_settlement_transaction_id_idx ON debts (settlement_transaction_id);
CREATE INDEX debts_original_amount_currency_id_idx ON debts (original_amount_currency_id);
CREATE INDEX debts_outstanding_currency_id_idx ON debts (outstanding_currency_id);

-- +migrate Down
-- Drop debts table and indexes

DROP TABLE IF EXISTS debts;