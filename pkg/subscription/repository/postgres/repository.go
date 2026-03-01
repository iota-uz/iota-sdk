package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/subscription/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) repository.Repository {
	return &Repository{pool: pool}
}

func (r *Repository) getQueryer(ctx context.Context) (queryer, error) {
	tx, err := composables.UseTx(ctx)
	if err == nil {
		return tx, nil
	}
	if r.pool == nil {
		return nil, composables.ErrNoPool
	}
	return r.pool, nil
}

func (r *Repository) GetEntitlement(ctx context.Context, tenantID uuid.UUID) (*repository.Entitlement, error) {
	const op serrors.Op = "SubscriptionRepository.GetEntitlement"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var model entitlementModel
	err = db.QueryRow(ctx, `
		SELECT tenant_id, plan_id, stripe_subscription_id, stripe_customer_id, features, entity_limits,
		       seat_limit, current_seats, in_grace_period, grace_period_ends_at, last_synced_at,
		       stripe_subscription_end, created_at, updated_at
		FROM subscription_entitlements
		WHERE tenant_id = $1
		`, tenantID).Scan(
		&model.TenantID,
		&model.PlanID,
		&model.StripeSubscriptionID,
		&model.StripeCustomerID,
		&model.Features,
		&model.EntityLimits,
		&model.SeatLimit,
		&model.CurrentSeats,
		&model.InGracePeriod,
		&model.GracePeriodEndsAt,
		&model.LastSyncedAt,
		&model.StripeSubscriptionEnd,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrEntitlementNotFound
		}
		return nil, serrors.E(op, err)
	}

	entitlement, err := toDomain(&model)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return entitlement, nil
}

func (r *Repository) UpsertEntitlement(ctx context.Context, entitlement *repository.Entitlement) error {
	const op serrors.Op = "SubscriptionRepository.UpsertEntitlement"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model, err := toModel(entitlement)
	if err != nil {
		return serrors.E(op, err)
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now().UTC()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = time.Now().UTC()
	}

	_, err = db.Exec(ctx, `
		INSERT INTO subscription_entitlements (
			tenant_id, plan_id, stripe_subscription_id, stripe_customer_id, features, entity_limits,
			seat_limit, current_seats, in_grace_period, grace_period_ends_at, last_synced_at,
			stripe_subscription_end, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (tenant_id) DO UPDATE SET
			plan_id = EXCLUDED.plan_id,
			stripe_subscription_id = EXCLUDED.stripe_subscription_id,
			stripe_customer_id = EXCLUDED.stripe_customer_id,
			features = EXCLUDED.features,
			entity_limits = EXCLUDED.entity_limits,
			seat_limit = EXCLUDED.seat_limit,
			current_seats = EXCLUDED.current_seats,
			in_grace_period = EXCLUDED.in_grace_period,
			grace_period_ends_at = EXCLUDED.grace_period_ends_at,
			last_synced_at = EXCLUDED.last_synced_at,
			stripe_subscription_end = EXCLUDED.stripe_subscription_end,
			updated_at = EXCLUDED.updated_at
		`,
		model.TenantID,
		model.PlanID,
		model.StripeSubscriptionID,
		model.StripeCustomerID,
		model.Features,
		model.EntityLimits,
		model.SeatLimit,
		model.CurrentSeats,
		model.InGracePeriod,
		model.GracePeriodEndsAt,
		model.LastSyncedAt,
		model.StripeSubscriptionEnd,
		model.CreatedAt,
		model.UpdatedAt,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

func (r *Repository) SetStripeReferences(ctx context.Context, tenantID uuid.UUID, customerID, subscriptionID *string) error {
	const op serrors.Op = "SubscriptionRepository.SetStripeReferences"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := db.Exec(ctx, `
		UPDATE subscription_entitlements
		SET stripe_customer_id = $2,
		    stripe_subscription_id = $3,
		    updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID, customerID, subscriptionID)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrEntitlementNotFound
	}
	return nil
}

func (r *Repository) FindTenantByStripeCustomer(ctx context.Context, customerID string) (uuid.UUID, error) {
	const op serrors.Op = "SubscriptionRepository.FindTenantByStripeCustomer"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return uuid.Nil, serrors.E(op, err)
	}

	var tenantID uuid.UUID
	err = db.QueryRow(ctx, `SELECT tenant_id FROM subscription_entitlements WHERE stripe_customer_id = $1`, customerID).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, repository.ErrEntitlementNotFound
		}
		return uuid.Nil, serrors.E(op, err)
	}
	return tenantID, nil
}

func (r *Repository) FindTenantByStripeSubscription(ctx context.Context, subscriptionID string) (uuid.UUID, error) {
	const op serrors.Op = "SubscriptionRepository.FindTenantByStripeSubscription"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return uuid.Nil, serrors.E(op, err)
	}

	var tenantID uuid.UUID
	err = db.QueryRow(ctx, `SELECT tenant_id FROM subscription_entitlements WHERE stripe_subscription_id = $1`, subscriptionID).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, repository.ErrEntitlementNotFound
		}
		return uuid.Nil, serrors.E(op, err)
	}
	return tenantID, nil
}

func (r *Repository) SetGracePeriod(ctx context.Context, tenantID uuid.UUID, inGrace bool, endsAt *time.Time) error {
	const op serrors.Op = "SubscriptionRepository.SetGracePeriod"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	result, err := db.Exec(ctx, `
		UPDATE subscription_entitlements
		SET in_grace_period = $2,
		    grace_period_ends_at = $3,
		    updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID, inGrace, endsAt)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrEntitlementNotFound
	}
	return nil
}

func (r *Repository) SetPlan(ctx context.Context, tenantID uuid.UUID, planID string) error {
	const op serrors.Op = "SubscriptionRepository.SetPlan"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	result, err := db.Exec(ctx, `
			UPDATE subscription_entitlements
			SET plan_id = $2,
		    updated_at = NOW()
		WHERE tenant_id = $1
		`, tenantID, planID)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrEntitlementNotFound
	}
	return nil
}

func (r *Repository) TouchSyncedAt(ctx context.Context, tenantID uuid.UUID, syncedAt time.Time) error {
	const op serrors.Op = "SubscriptionRepository.TouchSyncedAt"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	result, err := db.Exec(ctx, `
		UPDATE subscription_entitlements
		SET last_synced_at = $2,
		    updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID, syncedAt.UTC())
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrEntitlementNotFound
	}
	return nil
}

func (r *Repository) GetEntityCounts(ctx context.Context, tenantID uuid.UUID) (map[string]int, error) {
	const op serrors.Op = "SubscriptionRepository.GetEntityCounts"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := db.Query(ctx, `
		SELECT entity_type, current_count
		FROM subscription_entity_counts
		WHERE tenant_id = $1
	`, tenantID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var entityType string
		var current int
		if err := rows.Scan(&entityType, &current); err != nil {
			return nil, serrors.E(op, err)
		}
		counts[entityType] = current
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return counts, nil
}

func (r *Repository) GetEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string) (int, error) {
	const op serrors.Op = "SubscriptionRepository.GetEntityCount"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	var current int
	err = db.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT current_count
			FROM subscription_entity_counts
			WHERE tenant_id = $1 AND entity_type = $2
		), 0)
	`, tenantID, entityType).Scan(&current)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return current, nil
}

func (r *Repository) SetEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string, count int) error {
	const op serrors.Op = "SubscriptionRepository.SetEntityCount"
	if count < 0 {
		return serrors.E(op, fmt.Errorf("count must be non-negative"))
	}

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO subscription_entity_counts (tenant_id, entity_type, current_count, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id, entity_type) DO UPDATE
		SET current_count = EXCLUDED.current_count,
		    updated_at = NOW()
	`, tenantID, entityType, count)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (r *Repository) IncrementEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string) error {
	const op serrors.Op = "SubscriptionRepository.IncrementEntityCount"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO subscription_entity_counts (tenant_id, entity_type, current_count, updated_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (tenant_id, entity_type) DO UPDATE
		SET current_count = subscription_entity_counts.current_count + 1,
		    updated_at = NOW()
	`, tenantID, entityType)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (r *Repository) IncrementEntityCountIfBelow(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
	max int,
) (bool, error) {
	const op serrors.Op = "SubscriptionRepository.IncrementEntityCountIfBelow"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	var ok bool
	err = db.QueryRow(ctx, `
		WITH upsert AS (
			INSERT INTO subscription_entity_counts (tenant_id, entity_type, current_count, updated_at)
			SELECT $1, $2, 1, NOW()
			WHERE $3 > 0
			ON CONFLICT (tenant_id, entity_type) DO UPDATE
			SET current_count = subscription_entity_counts.current_count + 1,
			    updated_at = NOW()
			WHERE subscription_entity_counts.current_count < $3
			RETURNING current_count
		)
		SELECT EXISTS(SELECT 1 FROM upsert)
	`, tenantID, entityType, max).Scan(&ok)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return ok, nil
}

func (r *Repository) DecrementEntityCount(ctx context.Context, tenantID uuid.UUID, entityType string) error {
	const op serrors.Op = "SubscriptionRepository.DecrementEntityCount"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO subscription_entity_counts (tenant_id, entity_type, current_count, updated_at)
		VALUES ($1, $2, 0, NOW())
		ON CONFLICT (tenant_id, entity_type) DO UPDATE
		SET current_count = GREATEST(0, subscription_entity_counts.current_count - 1),
		    updated_at = NOW()
	`, tenantID, entityType)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (r *Repository) AddSeatIfBelow(ctx context.Context, tenantID uuid.UUID, max int) (bool, error) {
	const op serrors.Op = "SubscriptionRepository.AddSeatIfBelow"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	result, err := db.Exec(ctx, `
		UPDATE subscription_entitlements
		SET current_seats = current_seats + 1,
		    updated_at = NOW()
		WHERE tenant_id = $1
		  AND current_seats < $2
	`, tenantID, max)
	if err != nil {
		return false, serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return false, nil
	}
	return true, nil
}

func (r *Repository) IncrementSeat(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "SubscriptionRepository.IncrementSeat"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := db.Exec(ctx, `
		UPDATE subscription_entitlements
		SET current_seats = current_seats + 1,
		    updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrEntitlementNotFound
	}
	return nil
}

func (r *Repository) DecrementSeat(ctx context.Context, tenantID uuid.UUID) error {
	const op serrors.Op = "SubscriptionRepository.DecrementSeat"

	db, err := r.getQueryer(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := db.Exec(ctx, `
		UPDATE subscription_entitlements
		SET current_seats = GREATEST(0, current_seats - 1),
		    updated_at = NOW()
		WHERE tenant_id = $1
	`, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return repository.ErrEntitlementNotFound
	}
	return nil
}

func (r *Repository) UpsertPlans(ctx context.Context, plans []repository.Plan) error {
	const op serrors.Op = "SubscriptionRepository.UpsertPlans"

	if tx, err := composables.UseTx(ctx); err == nil {
		if err := r.upsertPlans(ctx, tx, plans); err != nil {
			return serrors.E(op, err)
		}
		return nil
	}

	if r.pool == nil {
		return serrors.E(op, composables.ErrNoPool)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return serrors.E(op, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := r.upsertPlans(ctx, tx, plans); err != nil {
		return serrors.E(op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return serrors.E(op, err)
	}
	committed = true
	return nil
}

func (r *Repository) upsertPlans(ctx context.Context, db queryer, plans []repository.Plan) error {
	for _, plan := range plans {
		m, err := planToModel(plan)
		if err != nil {
			return err
		}
		_, err = db.Exec(ctx, `
					INSERT INTO subscription_plans (
						plan_id, name, description, price_cents, billing_interval,
				features, entity_limits, seat_limit, display_order, is_active, is_public, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9, TRUE, TRUE, NOW()
				)
					ON CONFLICT (plan_id) DO UPDATE SET
					name = EXCLUDED.name,
					description = EXCLUDED.description,
					price_cents = EXCLUDED.price_cents,
				billing_interval = EXCLUDED.billing_interval,
				features = EXCLUDED.features,
				entity_limits = EXCLUDED.entity_limits,
				seat_limit = EXCLUDED.seat_limit,
					display_order = EXCLUDED.display_order,
					updated_at = NOW()
			`, m.PlanID, m.Name, m.Description, m.PriceCents, m.Interval, m.Features, m.EntityLimits, m.SeatLimit, m.DisplayOrder)
		if err != nil {
			return fmt.Errorf("upsert plan %s: %w", plan.PlanID, err)
		}
	}
	return nil
}
