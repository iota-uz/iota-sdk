-- +migrate Up
BEGIN;

CREATE TABLE counterparty
(
    id            SERIAL PRIMARY KEY,
    tin           VARCHAR(20),
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(255) NOT NULL, -- customer, supplier, individual
    legal_type    VARCHAR(255) NOT NULL, -- LLC, JSC, etc.
    legal_address VARCHAR(255),
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE counterparty_contacts
(
    id              SERIAL PRIMARY KEY,
    counterparty_id INT          NOT NULL REFERENCES counterparty (id) ON DELETE CASCADE,
    first_name      VARCHAR(255) NOT NULL,
    last_name       VARCHAR(255) NOT NULL,
    middle_name     VARCHAR(255) NULL,
    email           VARCHAR(255),
    phone           VARCHAR(255),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE inventory
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255)  NOT NULL,
    description TEXT,
    currency_id VARCHAR(3)    REFERENCES currencies (code) ON DELETE SET NULL,
    price       NUMERIC(9, 2) NOT NULL,
    quantity    INT           NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE expense_categories
(
    id                 SERIAL PRIMARY KEY,
    name               VARCHAR(255)  NOT NULL,
    description        TEXT,
    amount             NUMERIC(9, 2) NOT NULL,
    amount_currency_id VARCHAR(3)    NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);


CREATE TABLE money_accounts
(
    id                  SERIAL PRIMARY KEY,
    name                VARCHAR(255)  NOT NULL,
    account_number      VARCHAR(255)  NOT NULL,
    description         TEXT,
    balance             NUMERIC(9, 2) NOT NULL,
    balance_currency_id VARCHAR(3)    NOT NULL REFERENCES currencies (code) ON DELETE CASCADE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE transactions
(
    id                     SERIAL PRIMARY KEY,
    amount                 NUMERIC(9, 2) NOT NULL,
    origin_account_id      INT REFERENCES money_accounts (id) ON DELETE RESTRICT,
    destination_account_id INT REFERENCES money_accounts (id) ON DELETE RESTRICT,
    transaction_date       DATE          NOT NULL   DEFAULT CURRENT_DATE,
    accounting_period      DATE          NOT NULL   DEFAULT CURRENT_DATE,
    transaction_type       VARCHAR(255)  NOT NULL, -- income, expense, transfer
    comment                TEXT,
    created_at             TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE expenses
(
    id             SERIAL PRIMARY KEY,
    transaction_id INT NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    category_id    INT NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE payments
(
    id              SERIAL PRIMARY KEY,
    transaction_id  INT NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT,
    counterparty_id INT NOT NULL REFERENCES counterparty (id) ON DELETE RESTRICT,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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

COMMIT;

-- +migrate Down
BEGIN;

DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS money_accounts;
DROP TABLE IF EXISTS expense_categories;
DROP TABLE IF EXISTS inventory;
DROP TABLE IF EXISTS counterparty_contacts;
DROP TABLE IF EXISTS counterparty;

COMMIT;