package manage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// TriggerOwner is the user a trigger without a user (subscription on a
// system event, schedule, public endpoint) runs as, per tenant.
type TriggerOwner struct {
	TriggerID  string    `json:"triggerId"`
	TenantID   uuid.UUID `json:"tenantId"`
	UserID     int64     `json:"userId"`
	AssignedBy *int64    `json:"assignedBy,omitempty"`
	AssignedAt time.Time `json:"assignedAt"`
}

func (s *Service) triggerExists(ctx context.Context, id string) (bool, error) {
	active, err := s.catalog.Active(ctx)
	if err != nil {
		return false, err
	}
	for _, av := range active {
		for _, t := range av.Manifest.Subscriptions {
			if t.ID == id {
				return true, nil
			}
		}
		for _, t := range av.Manifest.Schedules {
			if t.ID == id {
				return true, nil
			}
		}
		for _, t := range av.Manifest.Endpoints {
			if t.ID == id {
				return true, nil
			}
		}
		// a contact resolver runs after commit without a user: it needs an owner too
		for _, i := range av.Manifest.Implementations {
			if i.Kind == manifest.ImplContactResolver && i.Implements == id {
				return true, nil
			}
		}
	}
	return false, nil
}

// AssignOwner sets the owner-user of a trigger in the actor's tenant. The
// trigger must be declared by an active package and the user must belong to
// the tenant. A tenant without an owner gets blocked trigger runs, not a
// failed activation.
func (s *Service) AssignOwner(ctx context.Context, actor spi.Subject, triggerID string, userID int64) (TriggerOwner, error) {
	const op = serrors.Op("extruntime.manage.AssignOwner")
	e := &auditEntry{Action: ActionAssignOwner, Target: triggerID, Detail: map[string]any{"tenantId": actor.TenantID.String(), "ownerUserId": userID}}
	var out TriggerOwner
	err := s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		ok, err := s.triggerExists(ctx, triggerID)
		if err != nil {
			return err
		}
		if !ok {
			return notFound("trigger " + triggerID)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND tenant_id = $2)`, userID, actor.TenantID).Scan(&exists); err != nil {
			return serrors.FromDB(op, err)
		}
		if !exists {
			return serrors.New(serrors.Invalid, "the owner must be a user of the tenant")
		}
		out = TriggerOwner{TriggerID: triggerID, TenantID: actor.TenantID, UserID: userID}
		return tx.QueryRow(ctx, `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id, assigned_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (trigger_id, tenant_id) DO UPDATE SET user_id = EXCLUDED.user_id, assigned_by = EXCLUDED.assigned_by, assigned_at = CURRENT_TIMESTAMP
			RETURNING assigned_by, assigned_at`, triggerID, actor.TenantID, userID, actor.UserID).Scan(&out.AssignedBy, &out.AssignedAt)
	})
	return out, serrors.Wrap(op, err)
}

// RemoveOwner clears the owner of a trigger in the actor's tenant.
func (s *Service) RemoveOwner(ctx context.Context, actor spi.Subject, triggerID string) error {
	e := &auditEntry{Action: ActionRemoveOwner, Target: triggerID, Detail: map[string]any{"tenantId": actor.TenantID.String()}}
	return s.mutate(ctx, actor, PermPublish, e, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM extruntime.trigger_owners WHERE trigger_id = $1 AND tenant_id = $2`, triggerID, actor.TenantID)
		if err != nil {
			return serrors.FromDB("extruntime.manage.RemoveOwner", err)
		}
		if tag.RowsAffected() == 0 {
			return notFound("owner of " + triggerID)
		}
		return nil
	})
}

// ListOwners lists the owners of the actor's tenant.
func (s *Service) ListOwners(ctx context.Context, actor spi.Subject) ([]TriggerOwner, error) {
	const op = serrors.Op("extruntime.manage.ListOwners")
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT trigger_id, tenant_id, user_id, assigned_by, assigned_at
		FROM extruntime.trigger_owners WHERE tenant_id = $1 ORDER BY trigger_id`, actor.TenantID)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []TriggerOwner
	for rows.Next() {
		var o TriggerOwner
		if err := rows.Scan(&o.TriggerID, &o.TenantID, &o.UserID, &o.AssignedBy, &o.AssignedAt); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, o)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// OwnerOf returns the owner-user of a trigger in a tenant for the events and
// workflow slices. ok is false when none is assigned (the run is blocked).
func (s *Service) OwnerOf(ctx context.Context, tenantID uuid.UUID, triggerID string) (userID int64, ok bool, err error) {
	err = s.db.QueryRow(ctx, `SELECT user_id FROM extruntime.trigger_owners WHERE trigger_id = $1 AND tenant_id = $2`, triggerID, tenantID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, serrors.FromDB("extruntime.manage.OwnerOf", err)
	}
	return userID, true, nil
}
