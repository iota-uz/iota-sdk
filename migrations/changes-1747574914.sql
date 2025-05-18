-- +migrate Up
-- Change CREATE_TABLE: billing_transactions
create table billing_transactions (
    id uuid default gen_random_uuid () primary key,
    status varchar(50) not null check (status in ('created', 'pending', 'completed', 'failed', 'canceled', 'refunded', 'partially-refunded', 'expired')),
    quantity float8 not null,
    currency varchar(3) not null check (currency in ('UZS', 'USD', 'EUR', 'RUB')),
    gateway varchar(50) not null check (gateway in ('click', 'payme', 'octo', 'stripe')),
    details jsonb not null,
    created_at timestamptz default NOW() not null,
    updated_at timestamptz default NOW() not null
);

-- Change CREATE_INDEX: idx_billing_transactions_gateway
create index idx_billing_transactions_gateway on billing_transactions (gateway);

-- Change CREATE_INDEX: idx_billing_transactions_status
create index idx_billing_transactions_status on billing_transactions (status);

-- +migrate Down
-- Undo CREATE_INDEX: idx_billing_transactions_status
drop index idx_billing_transactions_status;

-- Undo CREATE_INDEX: idx_billing_transactions_gateway
drop index idx_billing_transactions_gateway;

-- Undo CREATE_TABLE: billing_transactions
drop table if exists billing_transactions cascade;

