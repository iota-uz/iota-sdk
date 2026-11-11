package manage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SubjectLoader resolves the current rights of a user; host.SubjectLoader
// satisfies it.
type SubjectLoader interface {
	LoadSubject(ctx context.Context, tenantID uuid.UUID, userID int64, kind spi.SubjectKind) (spi.Subject, error)
}

// TriggerOwner is the user a trigger without a user (subscription on a
// system event, schedule, public endpoint) runs as, per tenant.
type TriggerOwner struct {
	TriggerID  string    `json:"triggerId"`
	TenantID   uuid.UUID `json:"tenantId"`
	UserID     int64     `json:"userId"`
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
		// a contact resolver and a delivery policy run after commit without a
		// user: they need an owner too
		for _, i := range av.Manifest.Implementations {
			if (i.Kind == manifest.ImplContactResolver || i.Kind == manifest.ImplDeliveryPolicy) && i.Implements == id {
				return true, nil
			}
		}
	}
	return false, nil
}

// checkOwnerRights refuses an owner who outranks the assigner. A trigger runs
// with the current rights of its owner, so naming a user is lending out their
// permissions: the actor may name themselves, or a user whose every current
// permission (direct, role, group) the actor holds too. Without a rights
// source only self-assignment can be proven safe.
func (s *Service) checkOwnerRights(ctx context.Context, actor spi.Subject, userID int64, detail map[string]any) error {
	const op = serrors.Op("extruntime.manage.checkOwnerRights")
	if userID == actor.UserID {
		detail["ownerRights"] = "self"
		return nil
	}
	denied := serrors.New(serrors.PermissionDenied, "the owner must be you or a user whose permissions you all hold").WithOp(op)
	if s.subjects == nil {
		detail["ownerRights"] = "unverifiable"
		return denied
	}
	owner, err := s.subjects.LoadSubject(ctx, actor.TenantID, userID, spi.SubjectOwner)
	if err != nil {
		detail["ownerRights"] = "unresolved"
		return serrors.New(serrors.Invalid, "the owner's rights cannot be resolved").WithOp(op).WithCause(err)
	}
	var excess int
	for _, p := range owner.Permissions {
		if !actor.Has(p) {
			excess++
		}
	}
	if excess > 0 {
		detail["ownerRights"] = "exceeds assigner"
		detail["excessPermissions"] = excess
		return denied
	}
	detail["ownerRights"] = "covered"
	return nil
}

// AssignOwner sets the owner-user of a trigger in the actor's tenant. The
// trigger must be declared by an active package, the user must belong to the
// tenant, and must not hold permissions the actor lacks (see
// checkOwnerRights). A tenant without an owner gets blocked trigger runs, not
// a failed activation.
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
		if err := s.checkOwnerRights(ctx, actor, userID, e.Detail); err != nil {
			return err
		}
		out = TriggerOwner{TriggerID: triggerID, TenantID: actor.TenantID, UserID: userID}
		return tx.QueryRow(ctx, `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (trigger_id, tenant_id) DO UPDATE SET user_id = EXCLUDED.user_id, assigned_at = CURRENT_TIMESTAMP
			RETURNING assigned_at`, triggerID, actor.TenantID, userID).Scan(&out.AssignedAt)
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
	rows, err := s.db.Query(ctx, `SELECT trigger_id, tenant_id, user_id, assigned_at
		FROM extruntime.trigger_owners WHERE tenant_id = $1 ORDER BY trigger_id`, actor.TenantID)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []TriggerOwner
	for rows.Next() {
		var o TriggerOwner
		if err := rows.Scan(&o.TriggerID, &o.TenantID, &o.UserID, &o.AssignedAt); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out = append(out, o)
	}
	return out, serrors.FromDB(op, rows.Err())
}

// OwnerOf returns the owner-user of a trigger in a tenant for the events and
// workflow slices. ok is false when none is assigned (the run is blocked).
func (s *Service) OwnerOf(ctx context.Context, tenantID uuid.UUID, triggerID string) (int64, bool, error) {
	return triggerowner.Lookup(ctx, s.db, triggerID, tenantID)
}
