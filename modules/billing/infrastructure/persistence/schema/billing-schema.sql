create table billing_transactions (
    id uuid primary key default gen_random_uuid (),
    status varchar(50) not null check (status in ('created', 'pending', 'completed', 'failed', 'canceled', 'refunded', 'partially-refunded', 'expired')),
    quantity float8 not null,
    currency varchar(3) not null check (currency in ('UZS', 'USD', 'EUR', 'RUB')),
    gateway varchar(50) not null check (gateway in ('click', 'payme', 'octo', 'stripe')),
    details jsonb not null,
    created_at timestamptz not null default NOW(),
    updated_at timestamptz not null default NOW()
);

create index idx_billing_transactions_status on billing_transactions (status);

create index idx_billing_transactions_gateway on billing_transactions (gateway);

