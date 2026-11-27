-- Migration: Add 'cash' and 'integrator' payment gateways to billing_transactions
-- Date: 2025-10-29
-- Purpose: Extend billing_transactions.gateway CHECK constraint to support cash and integrator payment methods

-- +migrate Up
-- Drop existing CHECK constraint on gateway field
ALTER TABLE billing_transactions
DROP CONSTRAINT IF EXISTS billing_transactions_gateway_check;

-- Add new CHECK constraint with all 6 gateway values
ALTER TABLE billing_transactions
ADD CONSTRAINT billing_transactions_gateway_check
CHECK (gateway IN ('click', 'payme', 'octo', 'stripe', 'cash', 'integrator'));

-- +migrate Down
-- Drop the updated CHECK constraint
ALTER TABLE billing_transactions
DROP CONSTRAINT IF EXISTS billing_transactions_gateway_check;

-- Restore original CHECK constraint with 4 gateway values
ALTER TABLE billing_transactions
ADD CONSTRAINT billing_transactions_gateway_check
CHECK (gateway IN ('click', 'payme', 'octo', 'stripe'));
