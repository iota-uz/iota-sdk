CREATE TABLE billing_transactions
(
    id         UUID PRIMARY KEY     DEFAULT gen_random_uuid(),
    status     VARCHAR(50) NOT NULL CHECK (
        status IN (
                   'created',
                   'pending',
                   'completed',
                   'failed',
                   'canceled',
                   'refunded',
                   'partially-refunded',
                   'expired'
            )
        ),
    quantity   FLOAT8      NOT NULL,
    currency   VARCHAR(3)  NOT NULL CHECK (
        currency IN ('UZS', 'USD', 'EUR', 'RUB')
        ),
    gateway    VARCHAR(50) NOT NULL CHECK (
        gateway IN ('click', 'payme', 'octo', 'stripe')
        ),
    details    JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_billing_transactions_status ON billing_transactions(status);
CREATE INDEX idx_billing_transactions_gateway ON billing_transactions(gateway);