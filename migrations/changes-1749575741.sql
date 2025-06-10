-- +migrate Up

-- First, drop all foreign key constraints that will be affected
ALTER TABLE counterparty_contacts DROP CONSTRAINT IF EXISTS counterparty_contacts_counterparty_id_fkey;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_counterparty_id_fkey;
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_category_id_fkey;
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_transaction_id_fkey;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_transaction_id_fkey;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_origin_account_id_fkey;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_destination_account_id_fkey;

-- Change ALTER_TABLE: counterparty - change id to uuid  
ALTER TABLE counterparty DROP CONSTRAINT counterparty_pkey;
ALTER TABLE counterparty ALTER COLUMN id DROP DEFAULT;
ALTER TABLE counterparty ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE counterparty ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE counterparty ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: counterparty_contacts - change id and foreign keys to uuid
ALTER TABLE counterparty_contacts DROP CONSTRAINT counterparty_contacts_pkey;
ALTER TABLE counterparty_contacts ALTER COLUMN id DROP DEFAULT;
ALTER TABLE counterparty_contacts ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE counterparty_contacts ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE counterparty_contacts ALTER COLUMN counterparty_id TYPE uuid USING gen_random_uuid();
ALTER TABLE counterparty_contacts ADD PRIMARY KEY (id);
ALTER TABLE counterparty_contacts ADD CONSTRAINT counterparty_contacts_counterparty_id_fkey 
    FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE CASCADE;

-- Change ALTER_TABLE: inventory - change id to uuid
ALTER TABLE inventory DROP CONSTRAINT inventory_pkey;
ALTER TABLE inventory ALTER COLUMN id DROP DEFAULT;
ALTER TABLE inventory ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE inventory ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE inventory ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: expense_categories - change id to uuid
ALTER TABLE expense_categories DROP CONSTRAINT expense_categories_pkey;
ALTER TABLE expense_categories ALTER COLUMN id DROP DEFAULT;
ALTER TABLE expense_categories ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE expense_categories ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE expense_categories ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: money_accounts - change id to uuid
ALTER TABLE money_accounts DROP CONSTRAINT money_accounts_pkey;
ALTER TABLE money_accounts ALTER COLUMN id DROP DEFAULT;
ALTER TABLE money_accounts ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE money_accounts ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE money_accounts ADD PRIMARY KEY (id);

-- Change ALTER_TABLE: transactions - change id and foreign keys to uuid, add exchange fields
ALTER TABLE transactions DROP CONSTRAINT transactions_pkey;
ALTER TABLE transactions ALTER COLUMN id DROP DEFAULT;
ALTER TABLE transactions ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE transactions ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE transactions ALTER COLUMN origin_account_id TYPE uuid USING gen_random_uuid();
ALTER TABLE transactions ALTER COLUMN destination_account_id TYPE uuid USING gen_random_uuid();
ALTER TABLE transactions ADD PRIMARY KEY (id);
ALTER TABLE transactions ADD CONSTRAINT transactions_origin_account_id_fkey 
    FOREIGN KEY (origin_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;
ALTER TABLE transactions ADD CONSTRAINT transactions_destination_account_id_fkey 
    FOREIGN KEY (destination_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;
-- Add exchange operation fields
ALTER TABLE transactions 
ADD COLUMN exchange_rate numeric(18, 8),
ADD COLUMN destination_amount numeric(9, 2);

-- Change ALTER_TABLE: expenses - change id and foreign keys to uuid
ALTER TABLE expenses DROP CONSTRAINT expenses_pkey;
ALTER TABLE expenses ALTER COLUMN id DROP DEFAULT;
ALTER TABLE expenses ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE expenses ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE expenses ALTER COLUMN transaction_id TYPE uuid USING gen_random_uuid();
ALTER TABLE expenses ALTER COLUMN category_id TYPE uuid USING gen_random_uuid();
ALTER TABLE expenses ADD PRIMARY KEY (id);
ALTER TABLE expenses ADD CONSTRAINT expenses_transaction_id_fkey 
    FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE CASCADE;
ALTER TABLE expenses ADD CONSTRAINT expenses_category_id_fkey 
    FOREIGN KEY (category_id) REFERENCES expense_categories (id) ON DELETE CASCADE;

-- Change CREATE_TABLE: payment_categories
CREATE TABLE payment_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, name)
);

-- Change CREATE_INDEX: idx_payment_categories_tenant_id
CREATE INDEX idx_payment_categories_tenant_id ON payment_categories (tenant_id);

-- Change ALTER_TABLE: payments - change id and foreign keys to uuid
ALTER TABLE payments DROP CONSTRAINT payments_pkey;
-- Note: foreign key constraints already dropped above
ALTER TABLE payments ALTER COLUMN id DROP DEFAULT;
ALTER TABLE payments ALTER COLUMN id TYPE uuid USING gen_random_uuid();
ALTER TABLE payments ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE payments ALTER COLUMN transaction_id TYPE uuid USING gen_random_uuid();
ALTER TABLE payments ALTER COLUMN counterparty_id TYPE uuid USING gen_random_uuid();
ALTER TABLE payments ADD PRIMARY KEY (id);
ALTER TABLE payments ADD CONSTRAINT payments_transaction_id_fkey 
    FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE RESTRICT;
ALTER TABLE payments ADD CONSTRAINT payments_counterparty_id_fkey 
    FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE RESTRICT;

-- +migrate Down
-- Undo payments table changes
ALTER TABLE payments DROP CONSTRAINT payments_pkey;
ALTER TABLE payments DROP CONSTRAINT payments_transaction_id_fkey;
ALTER TABLE payments DROP CONSTRAINT payments_counterparty_id_fkey;
ALTER TABLE payments ALTER COLUMN id TYPE serial;
ALTER TABLE payments ALTER COLUMN transaction_id TYPE int;
ALTER TABLE payments ALTER COLUMN counterparty_id TYPE int;
ALTER TABLE payments ADD PRIMARY KEY (id);
ALTER TABLE payments ADD CONSTRAINT payments_transaction_id_fkey 
    FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE RESTRICT;
ALTER TABLE payments ADD CONSTRAINT payments_counterparty_id_fkey 
    FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE RESTRICT;

-- Undo CREATE_INDEX: idx_payment_categories_tenant_id
DROP INDEX IF EXISTS idx_payment_categories_tenant_id;

-- Undo CREATE_TABLE: payment_categories
DROP TABLE IF EXISTS payment_categories CASCADE;

-- Undo expenses table changes
ALTER TABLE expenses DROP CONSTRAINT expenses_pkey;
ALTER TABLE expenses DROP CONSTRAINT expenses_transaction_id_fkey;
ALTER TABLE expenses DROP CONSTRAINT expenses_category_id_fkey;
ALTER TABLE expenses ALTER COLUMN id TYPE serial;
ALTER TABLE expenses ALTER COLUMN transaction_id TYPE int;
ALTER TABLE expenses ALTER COLUMN category_id TYPE int;
ALTER TABLE expenses ADD PRIMARY KEY (id);
ALTER TABLE expenses ADD CONSTRAINT expenses_transaction_id_fkey 
    FOREIGN KEY (transaction_id) REFERENCES transactions (id) ON DELETE CASCADE;
ALTER TABLE expenses ADD CONSTRAINT expenses_category_id_fkey 
    FOREIGN KEY (category_id) REFERENCES expense_categories (id) ON DELETE CASCADE;

-- Undo transactions table changes
ALTER TABLE transactions 
DROP COLUMN IF EXISTS destination_amount,
DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE transactions DROP CONSTRAINT transactions_pkey;
ALTER TABLE transactions DROP CONSTRAINT transactions_origin_account_id_fkey;
ALTER TABLE transactions DROP CONSTRAINT transactions_destination_account_id_fkey;
ALTER TABLE transactions ALTER COLUMN id TYPE serial;
ALTER TABLE transactions ALTER COLUMN origin_account_id TYPE int;
ALTER TABLE transactions ALTER COLUMN destination_account_id TYPE int;
ALTER TABLE transactions ADD PRIMARY KEY (id);
ALTER TABLE transactions ADD CONSTRAINT transactions_origin_account_id_fkey 
    FOREIGN KEY (origin_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;
ALTER TABLE transactions ADD CONSTRAINT transactions_destination_account_id_fkey 
    FOREIGN KEY (destination_account_id) REFERENCES money_accounts (id) ON DELETE RESTRICT;

-- Undo money_accounts table changes
ALTER TABLE money_accounts DROP CONSTRAINT money_accounts_pkey;  
ALTER TABLE money_accounts ALTER COLUMN id TYPE serial;
ALTER TABLE money_accounts ADD PRIMARY KEY (id);

-- Undo expense_categories table changes
ALTER TABLE expense_categories DROP CONSTRAINT expense_categories_pkey;
ALTER TABLE expense_categories ALTER COLUMN id TYPE serial;
ALTER TABLE expense_categories ADD PRIMARY KEY (id);

-- Undo inventory table changes
ALTER TABLE inventory DROP CONSTRAINT inventory_pkey;
ALTER TABLE inventory ALTER COLUMN id TYPE serial;
ALTER TABLE inventory ADD PRIMARY KEY (id);

-- Undo counterparty_contacts table changes
ALTER TABLE counterparty_contacts DROP CONSTRAINT counterparty_contacts_pkey;
ALTER TABLE counterparty_contacts DROP CONSTRAINT counterparty_contacts_counterparty_id_fkey;
ALTER TABLE counterparty_contacts ALTER COLUMN id TYPE serial;
ALTER TABLE counterparty_contacts ALTER COLUMN counterparty_id TYPE int;
ALTER TABLE counterparty_contacts ADD PRIMARY KEY (id);
ALTER TABLE counterparty_contacts ADD CONSTRAINT counterparty_contacts_counterparty_id_fkey 
    FOREIGN KEY (counterparty_id) REFERENCES counterparty (id) ON DELETE CASCADE;

-- Undo counterparty table changes
ALTER TABLE counterparty DROP CONSTRAINT counterparty_pkey;
ALTER TABLE counterparty ALTER COLUMN id TYPE serial;
ALTER TABLE counterparty ADD PRIMARY KEY (id);