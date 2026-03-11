package seed

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/sirupsen/logrus"
)

const seedSubscriptionEntitlementsQuery = `
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
`

var createSubscriptionEntitlements = application.Seed(
	func(ctx context.Context, logger logrus.FieldLogger) error {
		db, err := composables.UseTx(ctx)
		if err != nil {
			logger.Errorf("Failed to get db transaction for subscription entitlement seed: %v", err)
			return err
		}
		logger.Info("Seeding subscription entitlements")
		if _, err = db.Exec(ctx, seedSubscriptionEntitlementsQuery); err != nil {
			logger.Errorf("Failed to seed subscription entitlements: %v", err)
			return err
		}
		return nil
	},
)

func CreateSubscriptionEntitlements(ctx context.Context, deps *application.SeedDeps) error {
	return createSubscriptionEntitlements(ctx, deps)
}
