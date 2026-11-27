-- Migration: Add file attachment functionality to finance module
-- Date: 2025-01-13
-- Purpose: Create junction tables for payment and expense attachments with proper foreign key relationships

-- +migrate Up

-- Create payment_attachments junction table
CREATE TABLE payment_attachments (
    payment_id uuid NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    upload_id bigint NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    attached_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    attached_by bigint REFERENCES users (id) ON DELETE SET NULL,
    PRIMARY KEY (payment_id, upload_id)
);

-- Create expense_attachments junction table
CREATE TABLE expense_attachments (
    expense_id uuid NOT NULL REFERENCES expenses (id) ON DELETE CASCADE,
    upload_id bigint NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    attached_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    attached_by bigint REFERENCES users (id) ON DELETE SET NULL,
    PRIMARY KEY (expense_id, upload_id)
);

-- Create performance indexes for payment attachments
CREATE INDEX idx_payment_attachments_payment ON payment_attachments(payment_id);
CREATE INDEX idx_payment_attachments_upload ON payment_attachments(upload_id);
CREATE INDEX idx_payment_attachments_attached_by ON payment_attachments(attached_by);

-- Create performance indexes for expense attachments
CREATE INDEX idx_expense_attachments_expense ON expense_attachments(expense_id);
CREATE INDEX idx_expense_attachments_upload ON expense_attachments(upload_id);
CREATE INDEX idx_expense_attachments_attached_by ON expense_attachments(attached_by);

-- +migrate Down

-- Drop indexes for expense attachments
DROP INDEX IF EXISTS idx_expense_attachments_attached_by;
DROP INDEX IF EXISTS idx_expense_attachments_upload;
DROP INDEX IF EXISTS idx_expense_attachments_expense;

-- Drop indexes for payment attachments
DROP INDEX IF EXISTS idx_payment_attachments_attached_by;
DROP INDEX IF EXISTS idx_payment_attachments_upload;
DROP INDEX IF EXISTS idx_payment_attachments_payment;

-- Drop junction tables
DROP TABLE IF EXISTS expense_attachments CASCADE;
DROP TABLE IF EXISTS payment_attachments CASCADE;