-- +migrate Up
CREATE TABLE IF NOT EXISTS subscription_entitlements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL UNIQUE REFERENCES tenants (id) ON DELETE CASCADE,
    plan_id varchar(20) NOT NULL DEFAULT 'FREE',
    stripe_subscription_id varchar(255),
    stripe_customer_id varchar(255),
    features jsonb NOT NULL DEFAULT '[]'::jsonb,
    entity_limits jsonb NOT NULL DEFAULT '{}'::jsonb,
    seat_limit int,
    current_seats int NOT NULL DEFAULT 0,
    in_grace_period boolean NOT NULL DEFAULT false,
    grace_period_ends_at timestamptz,
    last_synced_at timestamptz,
    stripe_subscription_end timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS subscription_entity_counts (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    entity_type varchar(50) NOT NULL,
    current_count int NOT NULL DEFAULT 0,
    period_start date,
    period_end date,
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, entity_type)
);

CREATE TABLE IF NOT EXISTS subscription_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id varchar(20) NOT NULL UNIQUE,
    name varchar(100) NOT NULL,
    description text,
    stripe_product_id varchar(255),
    stripe_price_id varchar(255),
    price_cents bigint NOT NULL DEFAULT 0,
    billing_interval varchar(20) NOT NULL DEFAULT 'month',
    features jsonb NOT NULL DEFAULT '[]'::jsonb,
    entity_limits jsonb NOT NULL DEFAULT '{}'::jsonb,
    seat_limit int,
    grace_period_days int NOT NULL DEFAULT 7,
    display_order int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    is_public boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscription_entitlements_plan_id ON subscription_entitlements (plan_id);
CREATE INDEX IF NOT EXISTS idx_subscription_entitlements_customer_id ON subscription_entitlements (stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_subscription_entitlements_subscription_id ON subscription_entitlements (stripe_subscription_id);

INSERT INTO subscription_entitlements (
    tenant_id,
    plan_id,
    features,
    entity_limits,
    current_seats
)
SELECT
    t.id,
    'FREE',
    '[]'::jsonb,
    '{}'::jsonb,
    COALESCE((SELECT COUNT(1) FROM users u WHERE u.tenant_id = t.id), 0)
FROM tenants t
ON CONFLICT (tenant_id) DO NOTHING;

-- +migrate Down
DROP INDEX IF EXISTS idx_subscription_entitlements_subscription_id;
DROP INDEX IF EXISTS idx_subscription_entitlements_customer_id;
DROP INDEX IF EXISTS idx_subscription_entitlements_plan_id;

DROP TABLE IF EXISTS subscription_plans;
DROP TABLE IF EXISTS subscription_entity_counts;
DROP TABLE IF EXISTS subscription_entitlements;
