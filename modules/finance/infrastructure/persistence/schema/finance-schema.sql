CREATE TABLE counterparty (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    tin varchar(20),
    name varchar(255) NOT NULL,
    type VARCHAR(255) NOT NULL, -- customer, supplier, individual
    legal_type varchar(255) NOT NULL, -- LLC, JSC, etc.
    legal_address varchar(255),
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT counterparty_tenant_tin_key UNIQUE (tenant_id, tin)
);

CREATE TABLE counterparty_contacts (
    id serial PRIMARY KEY,
    counterparty_id int NOT NULL REFERENCES counterparty (id) ON DELETE CASCADE,
    first_name varchar(255) NOT NULL,
    last_name varchar(255) NOT NULL,
    middle_name varchar(255) NULL,
    email varchar(255),
    phone varchar(255),
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE inventory (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    currency_id varchar(3) REFERENCES currencies (code) ON DELETE SET NULL,
    price numeric(9, 2) NOT NULL,
    quantity int NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT inventory_tenant_name_key UNIQUE (tenant_id, name)
);

CREATE TABLE expense_categories (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    amount numeric(9, 2) NOT NULL,
    amount_currency_id varchar(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT expense_categories_tenant_name_key UNIQUE (tenant_id, name)
);

CREATE TABLE money_accounts (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    account_number varchar(255) NOT NULL,
    description text,
    balance numeric(9, 2) NOT NULL,
    balance_currency_id varchar(3) NOT NULL REFERENCES currencies (code) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT money_accounts_tenant_account_number_key UNIQUE (tenant_id, account_number)
);

CREATE TABLE transactions (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    amount numeric(9, 2) NOT NULL,
    origin_account_id int REFERENCES money_accounts (id) ON DELETE RESTRICT,
    destination_account_id int REFERENCES money_accounts (id) ON DELETE RESTRICT,
    transaction_date date NOT NULL DEFAULT now() ::date,
    accounting_period date NOT NULL DEFAULT now() ::date,
    transaction_type varchar(255) NOT NULL, -- income, expense, transfer
    comment text,
    created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE expenses (
    id serial PRIMARY KEY,
    transaction_id int NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    category_id int NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE payments (
    id serial PRIMARY KEY,
    transaction_id int NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT,
    counterparty_id int NOT NULL REFERENCES counterparty (id) ON DELETE RESTRICT,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE INDEX expenses_category_id_idx ON expenses (category_id);

CREATE INDEX expenses_transaction_id_idx ON expenses (transaction_id);

CREATE INDEX payments_counterparty_id_idx ON payments (counterparty_id);

CREATE INDEX payments_transaction_id_idx ON payments (transaction_id);

CREATE INDEX transactions_destination_account_id_idx ON transactions (destination_account_id);

CREATE INDEX transactions_origin_account_id_idx ON transactions (origin_account_id);

CREATE INDEX counterparty_contacts_counterparty_id_idx ON counterparty_contacts (counterparty_id);

CREATE INDEX counterparty_tin_idx ON counterparty (tin);

CREATE INDEX inventory_currency_id_idx ON inventory (currency_id);

CREATE INDEX money_accounts_balance_currency_id_idx ON money_accounts (balance_currency_id);

CREATE INDEX transactions_tenant_id_idx ON transactions (tenant_id)

