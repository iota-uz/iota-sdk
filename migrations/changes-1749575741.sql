-- +migrate Up
-- First, drop all foreign key constraints that will be affected
ALTER TABLE counterparty_contacts
    DROP CONSTRAINT IF EXISTS counterparty_contacts_counterparty_id_fkey;

ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_counterparty_id_fkey;

ALTER TABLE expenses
    DROP CONSTRAINT IF EXISTS expenses_category_id_fkey;

ALTER TABLE expenses
    DROP CONSTRAINT IF EXISTS expenses_transaction_id_fkey;

ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_transaction_id_fkey;

ALTER TABLE payments
    ADD COLUMN tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS transactions_origin_account_id_fkey;

ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS transactions_destination_account_id_fkey;

-- Change ALTER_TABLE: counterparty - change id to uuid
ALTER TABLE counterparty
    DROP CONSTRAINT counterparty_pkey;

ALTER TABLE counterparty
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE counterparty
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE counterparty
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE counterparty
    ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: counterparty_contacts - change id and foreign keys to uuid
ALTER TABLE counterparty_contacts
    DROP CONSTRAINT counterparty_contacts_pkey;

ALTER TABLE counterparty_contacts
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE counterparty_contacts
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE counterparty_contacts
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE counterparty_contacts
    ALTER COLUMN counterparty_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE counterparty_contacts
    ADD PRIMARY KEY (id);

ALTER TABLE counterparty_contacts
    ADD CONSTRAINT counterparty_contacts_counterparty_id_fkey FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE CASCADE;

-- Change ALTER_TABLE: inventory - change id to uuid
ALTER TABLE inventory
    DROP CONSTRAINT inventory_pkey;

ALTER TABLE inventory
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE inventory
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE inventory
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE inventory
    ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: expense_categories - change id to uuid
ALTER TABLE expense_categories
    DROP CONSTRAINT expense_categories_pkey;

ALTER TABLE expense_categories
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE expense_categories
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE expense_categories
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE expense_categories
    ADD PRIMARY KEY (id);

-- Remove amount and currency fields from expense_categories
ALTER TABLE expense_categories
    DROP COLUMN IF EXISTS amount,
    DROP COLUMN IF EXISTS amount_currency_id;

-- Change ALTER_TABLE: money_accounts - change id to uuid
ALTER TABLE money_accounts
    DROP CONSTRAINT money_accounts_pkey;

ALTER TABLE money_accounts
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE money_accounts
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE money_accounts
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE money_accounts
    ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: transactions - change id and foreign keys to uuid, add exchange fields
ALTER TABLE transactions
    DROP CONSTRAINT transactions_pkey;

ALTER TABLE transactions
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE transactions
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE transactions
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE transactions
    ALTER COLUMN origin_account_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE transactions
    ALTER COLUMN destination_account_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE transactions
    ADD PRIMARY KEY (id);

ALTER TABLE transactions
    ADD CONSTRAINT transactions_origin_account_id_fkey FOREIGN KEY (origin_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_destination_account_id_fkey FOREIGN KEY (destination_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;

-- Add exchange operation fields and convert amounts to BIGINT for money package compatibility
ALTER TABLE transactions
    ADD COLUMN exchange_rate numeric(18, 8),
    ADD COLUMN destination_amount bigint;

-- Convert existing amount fields to BIGINT (store amounts as smallest currency unit)
ALTER TABLE transactions
    ALTER COLUMN amount TYPE bigint
    USING (amount * 100)::bigint;

ALTER TABLE money_accounts
    ALTER COLUMN balance TYPE bigint
    USING (balance * 100)::bigint;

ALTER TABLE inventory
    ALTER COLUMN price TYPE bigint
    USING (price * 100)::bigint;

-- Change ALTER_TABLE: expenses - change id and foreign keys to uuid
ALTER TABLE expenses
    DROP CONSTRAINT expenses_pkey;

ALTER TABLE expenses
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE expenses
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE expenses
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE expenses
    ALTER COLUMN transaction_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE expenses
    ALTER COLUMN transaction_id SET DEFAULT gen_random_uuid ();

ALTER TABLE expenses
    ALTER COLUMN category_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE expenses
    ALTER COLUMN category_id SET DEFAULT gen_random_uuid ();

ALTER TABLE expenses
    ADD PRIMARY KEY (id);

ALTER TABLE expenses
    ADD CONSTRAINT expenses_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE CASCADE;

ALTER TABLE expenses
    ADD CONSTRAINT expenses_category_id_fkey FOREIGN KEY (category_id) REFERENCES expense_categories (id) ON DELETE CASCADE;

-- Add tenant_id column to expenses table
ALTER TABLE expenses
    ADD COLUMN tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change CREATE_TABLE: payment_categories
CREATE TABLE payment_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, name)
);

-- Change CREATE_INDEX: idx_payment_categories_tenant_id
CREATE INDEX idx_payment_categories_tenant_id ON payment_categories (tenant_id);

-- Add payment_category_id column to payments table
ALTER TABLE payments
    ADD COLUMN payment_category_id uuid REFERENCES payment_categories (id) ON DELETE SET NULL;

-- Change ALTER_TABLE: payments - change id and foreign keys to uuid
ALTER TABLE payments
    DROP CONSTRAINT payments_pkey;

-- Note: foreign key constraints already dropped above
ALTER TABLE payments
    ALTER COLUMN id DROP DEFAULT;

ALTER TABLE payments
    ALTER COLUMN id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE payments
    ALTER COLUMN id SET DEFAULT gen_random_uuid ();

ALTER TABLE payments
    ALTER COLUMN transaction_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE payments
    ALTER COLUMN counterparty_id TYPE uuid
    USING gen_random_uuid ();

ALTER TABLE payments
    ADD PRIMARY KEY (id);

ALTER TABLE payments
    ADD CONSTRAINT payments_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE RESTRICT;

ALTER TABLE payments
    ADD CONSTRAINT payments_counterparty_id_fkey FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE RESTRICT;

-- +migrate Down
-- Undo payments table changes
ALTER TABLE payments
    DROP COLUMN IF EXISTS payment_category_id;

ALTER TABLE payments
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE payments
    DROP CONSTRAINT payments_pkey;

ALTER TABLE payments
    DROP CONSTRAINT payments_transaction_id_fkey;

ALTER TABLE payments
    DROP CONSTRAINT payments_counterparty_id_fkey;

CREATE SEQUENCE IF NOT EXISTS payments_id_seq;

ALTER TABLE payments
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE payments
    ALTER COLUMN id SET DEFAULT nextval('payments_id_seq');

ALTER SEQUENCE payments_id_seq OWNED BY payments.id;

ALTER TABLE payments
    ALTER COLUMN transaction_id TYPE int;

ALTER TABLE payments
    ALTER COLUMN counterparty_id TYPE int;

ALTER TABLE payments
    ADD PRIMARY KEY (id);

ALTER TABLE payments
    ADD CONSTRAINT payments_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE RESTRICT;

ALTER TABLE payments
    ADD CONSTRAINT payments_counterparty_id_fkey FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE RESTRICT;

-- Undo CREATE_INDEX: idx_payment_categories_tenant_id
DROP INDEX IF EXISTS idx_payment_categories_tenant_id;

-- Undo CREATE_TABLE: payment_categories
DROP TABLE IF EXISTS payment_categories CASCADE;

-- Undo expenses table changes
-- Remove tenant_id column from expenses table
ALTER TABLE expenses
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE expenses
    DROP CONSTRAINT expenses_pkey;

ALTER TABLE expenses
    DROP CONSTRAINT expenses_transaction_id_fkey;

ALTER TABLE expenses
    DROP CONSTRAINT expenses_category_id_fkey;

CREATE SEQUENCE IF NOT EXISTS expenses_id_seq;

ALTER TABLE expenses
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE expenses
    ALTER COLUMN id SET DEFAULT nextval('expenses_id_seq');

ALTER SEQUENCE expenses_id_seq OWNED BY expenses.id;

ALTER TABLE expenses
    ALTER COLUMN transaction_id TYPE int;

ALTER TABLE expenses
    ALTER COLUMN category_id TYPE int;

ALTER TABLE expenses
    ADD PRIMARY KEY (id);

ALTER TABLE expenses
    ADD CONSTRAINT expenses_transaction_id_fkey FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE CASCADE;

ALTER TABLE expenses
    ADD CONSTRAINT expenses_category_id_fkey FOREIGN KEY (category_id) REFERENCES expense_categories (id) ON DELETE CASCADE;

-- Undo transactions table changes
ALTER TABLE transactions
    DROP COLUMN IF EXISTS destination_amount,
    DROP COLUMN IF EXISTS exchange_rate;

-- Revert amount fields back to NUMERIC
ALTER TABLE transactions
    ALTER COLUMN amount TYPE numeric(9, 2)
    USING (amount / 100.0)::numeric(9, 2);

ALTER TABLE money_accounts
    ALTER COLUMN balance TYPE numeric(9, 2)
    USING (balance / 100.0)::numeric(9, 2);

ALTER TABLE inventory
    ALTER COLUMN price TYPE numeric(9, 2)
    USING (price / 100.0)::numeric(9, 2);

ALTER TABLE transactions
    DROP CONSTRAINT transactions_pkey;

ALTER TABLE transactions
    DROP CONSTRAINT transactions_origin_account_id_fkey;

ALTER TABLE transactions
    DROP CONSTRAINT transactions_destination_account_id_fkey;

CREATE SEQUENCE IF NOT EXISTS transactions_id_seq;

ALTER TABLE transactions
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE transactions
    ALTER COLUMN id SET DEFAULT nextval('transactions_id_seq');

ALTER SEQUENCE transactions_id_seq OWNED BY transactions.id;

ALTER TABLE transactions
    ALTER COLUMN origin_account_id TYPE int;

ALTER TABLE transactions
    ALTER COLUMN destination_account_id TYPE int;

ALTER TABLE transactions
    ADD PRIMARY KEY (id);

ALTER TABLE transactions
    ADD CONSTRAINT transactions_origin_account_id_fkey FOREIGN KEY (origin_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_destination_account_id_fkey FOREIGN KEY (destination_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;

-- Undo money_accounts table changes
ALTER TABLE money_accounts
    DROP CONSTRAINT money_accounts_pkey;

CREATE SEQUENCE IF NOT EXISTS money_accounts_id_seq;

ALTER TABLE money_accounts
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE money_accounts
    ALTER COLUMN id SET DEFAULT nextval('money_accounts_id_seq');

ALTER SEQUENCE money_accounts_id_seq OWNED BY money_accounts.id;

ALTER TABLE money_accounts
    ADD PRIMARY KEY (id);

-- Undo expense_categories table changes
-- Re-add amount and currency fields to expense_categories
ALTER TABLE expense_categories
    ADD COLUMN amount numeric(9, 2),
    ADD COLUMN amount_currency_id varchar(3);

ALTER TABLE expense_categories
    DROP CONSTRAINT expense_categories_pkey;

CREATE SEQUENCE IF NOT EXISTS expense_categories_id_seq;

ALTER TABLE expense_categories
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE expense_categories
    ALTER COLUMN id SET DEFAULT nextval('expense_categories_id_seq');

ALTER SEQUENCE expense_categories_id_seq OWNED BY expense_categories.id;

ALTER TABLE expense_categories
    ADD PRIMARY KEY (id);

-- Undo inventory table changes
ALTER TABLE inventory
    DROP CONSTRAINT inventory_pkey;

CREATE SEQUENCE IF NOT EXISTS inventory_id_seq;

ALTER TABLE inventory
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE inventory
    ALTER COLUMN id SET DEFAULT nextval('inventory_id_seq');

ALTER SEQUENCE inventory_id_seq OWNED BY inventory.id;

ALTER TABLE inventory
    ADD PRIMARY KEY (id);

-- Undo counterparty_contacts table changes
ALTER TABLE counterparty_contacts
    DROP CONSTRAINT counterparty_contacts_pkey;

ALTER TABLE counterparty_contacts
    DROP CONSTRAINT counterparty_contacts_counterparty_id_fkey;

CREATE SEQUENCE IF NOT EXISTS counterparty_contacts_id_seq;

ALTER TABLE counterparty_contacts
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE counterparty_contacts
    ALTER COLUMN id SET DEFAULT nextval('counterparty_contacts_id_seq');

ALTER SEQUENCE counterparty_contacts_id_seq OWNED BY counterparty_contacts.id;

ALTER TABLE counterparty_contacts
    ALTER COLUMN counterparty_id TYPE int;

ALTER TABLE counterparty_contacts
    ADD PRIMARY KEY (id);

ALTER TABLE counterparty_contacts
    ADD CONSTRAINT counterparty_contacts_counterparty_id_fkey FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE CASCADE;

-- Undo counterparty table changes
ALTER TABLE counterparty
    DROP CONSTRAINT counterparty_pkey;

CREATE SEQUENCE IF NOT EXISTS counterparty_id_seq;

ALTER TABLE counterparty
    ALTER COLUMN id TYPE INTEGER;

ALTER TABLE counterparty
    ALTER COLUMN id SET DEFAULT nextval('counterparty_id_seq');

ALTER SEQUENCE counterparty_id_seq OWNED BY counterparty.id;

ALTER TABLE counterparty
    ADD PRIMARY KEY (id);

