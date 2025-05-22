-- +migrate Up
-- Change CREATE_TABLE: billing_transactions
CREATE TABLE billing_transactions (
    id uuid DEFAULT gen_random_uuid () PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    status varchar(50) NOT NULL CHECK (status IN ('created', 'pending', 'completed', 'failed', 'canceled', 'refunded', 'partially-refunded', 'expired')),
    quantity float8 NOT NULL,
    currency varchar(3) NOT NULL CHECK (currency IN ('UZS', 'USD', 'EUR', 'RUB')),
    gateway varchar(50) NOT NULL CHECK (gateway IN ('click', 'payme', 'octo', 'stripe')),
    details jsonb NOT NULL,
    created_at timestamptz DEFAULT NOW() NOT NULL,
    updated_at timestamptz DEFAULT NOW() NOT NULL
);

-- Change CREATE_INDEX: idx_billing_transactions_gateway
CREATE INDEX idx_billing_transactions_gateway ON billing_transactions (gateway);

-- Change CREATE_INDEX: idx_billing_transactions_status
CREATE INDEX idx_billing_transactions_status ON billing_transactions (status);

-- Change CREATE_INDEX: idx_billing_transactions_tenant_id
CREATE INDEX idx_billing_transactions_tenant_id ON billing_transactions (tenant_id);

-- +migrate Down
-- Undo CREATE_INDEX: idx_billing_transactions_status
DROP INDEX idx_billing_transactions_status;

-- Undo CREATE_INDEX: idx_billing_transactions_gateway
DROP INDEX idx_billing_transactions_gateway;

-- Undo CREATE_TABLE: billing_transactions
DROP TABLE IF EXISTS billing_transactions CASCADE;

