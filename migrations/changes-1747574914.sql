-- +migrate Up

-- Change CREATE_TABLE: billing_transactions
CREATE TABLE billing_transactions (
	id         UUID DEFAULT gen_random_uuid() PRIMARY KEY,
	status     VARCHAR(50)
	           NOT NULL
	           CHECK (
				status
				IN (
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
	quantity   FLOAT8 NOT NULL,
	currency   VARCHAR(3) NOT NULL CHECK (currency IN ('UZS', 'USD', 'EUR', 'RUB')),
	gateway    VARCHAR(50) NOT NULL CHECK (gateway IN ('click', 'payme', 'octo', 'stripe')),
	details    JSONB NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
	updated_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

-- Change CREATE_INDEX: idx_billing_transactions_gateway
CREATE INDEX idx_billing_transactions_gateway ON billing_transactions (gateway);

-- Change CREATE_INDEX: idx_billing_transactions_status
CREATE INDEX idx_billing_transactions_status ON billing_transactions (status);


-- +migrate Down

-- Undo CREATE_INDEX: idx_billing_transactions_status
DROP INDEX idx_billing_transactions_status;

-- Undo CREATE_INDEX: idx_billing_transactions_gateway
DROP INDEX idx_billing_transactions_gateway;

-- Undo CREATE_TABLE: billing_transactions
DROP TABLE IF EXISTS billing_transactions CASCADE;

