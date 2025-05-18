CREATE TABLE billing_transactions (
                                      id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
                                      status varchar(50) NOT NULL CHECK (status IN ('created', 'pending', 'completed', 'failed', 'canceled', 'refunded', 'partially-refunded', 'expired')),
                                      quantity float8 NOT NULL,
                                      currency varchar(3) NOT NULL CHECK (currency IN ('UZS', 'USD', 'EUR', 'RUB')),
                                      gateway varchar(50) NOT NULL CHECK (gateway IN ('click', 'payme', 'octo', 'stripe')),
                                      details jsonb NOT NULL,
                                      created_at timestamptz NOT NULL DEFAULT NOW(),
                                      updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_transactions_status ON billing_transactions (status);

CREATE INDEX idx_billing_transactions_gateway ON billing_transactions (gateway);

