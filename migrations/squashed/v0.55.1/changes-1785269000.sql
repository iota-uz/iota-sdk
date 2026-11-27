-- Extend billing gateway constraint for transfer and Uzum transactions.
-- +migrate Up
ALTER TABLE billing_transactions
    DROP CONSTRAINT IF EXISTS billing_transactions_gateway_check;

ALTER TABLE billing_transactions
    ADD CONSTRAINT billing_transactions_gateway_check CHECK (gateway IN ('click', 'payme', 'octo', 'stripe', 'cash', 'transfer', 'integrator', 'uzum'));

-- +migrate Down
ALTER TABLE billing_transactions
    DROP CONSTRAINT IF EXISTS billing_transactions_gateway_check;

ALTER TABLE billing_transactions
    ADD CONSTRAINT billing_transactions_gateway_check CHECK (gateway IN ('click', 'payme', 'octo', 'stripe', 'cash', 'integrator'));
