-- Migration: Add 'transfer' payment gateway to billing_transactions
-- Date: 2026-05-06
-- Purpose: Extend billing_transactions.gateway CHECK constraint to allow the
-- new bank-transfer gateway introduced alongside the TransferDetails aggregate.

-- +migrate Up
ALTER TABLE billing_transactions
DROP CONSTRAINT IF EXISTS billing_transactions_gateway_check;

ALTER TABLE billing_transactions
ADD CONSTRAINT billing_transactions_gateway_check
CHECK (gateway IN ('click', 'payme', 'octo', 'stripe', 'cash', 'integrator', 'transfer'));

-- +migrate Down
ALTER TABLE billing_transactions
DROP CONSTRAINT IF EXISTS billing_transactions_gateway_check;

ALTER TABLE billing_transactions
ADD CONSTRAINT billing_transactions_gateway_check
CHECK (gateway IN ('click', 'payme', 'octo', 'stripe', 'cash', 'integrator'));
