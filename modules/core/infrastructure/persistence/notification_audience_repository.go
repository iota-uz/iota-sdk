package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationAudienceRepository struct{}

func NewNotificationAudienceRepository() notifications.AudienceRepository {
	return &NotificationAudienceRepository{}
}
func (r *NotificationAudienceRepository) Groups(ctx context.Context) ([]notifications.GroupOption, error) {
	const op = "NotificationAudienceRepository.Groups"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, "SELECT id,name FROM user_groups WHERE tenant_id=$1 ORDER BY name,id", tenant)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.GroupOption{}
	for rows.Next() {
		var option notifications.GroupOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, option)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}
func (r *NotificationAudienceRepository) Roles(ctx context.Context) ([]notifications.RoleOption, error) {
	const op = "NotificationAudienceRepository.Roles"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, "SELECT id,name FROM roles WHERE tenant_id=$1 ORDER BY name,id", tenant)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.RoleOption{}
	for rows.Next() {
		var option notifications.RoleOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, option)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}
func (r *NotificationAudienceRepository) Resolve(ctx context.Context, groups []uuid.UUID, roles []uint) ([]uint, error) {
	const op = "NotificationAudienceRepository.Resolve"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	roleIDs := make([]int64, len(roles))
	for i, id := range roles {
		roleIDs[i] = int64(id)
	}
	rows, err := db.Query(ctx, `SELECT DISTINCT u.id FROM users u WHERE u.tenant_id=$1 AND (
 EXISTS (SELECT 1 FROM group_users gu JOIN user_groups g ON g.id=gu.group_id WHERE gu.user_id=u.id AND g.tenant_id=$1 AND g.id=ANY($2::uuid[]))
 OR EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=u.id AND r.tenant_id=$1 AND r.id=ANY($3::bigint[]))
 OR EXISTS (SELECT 1 FROM group_users gu JOIN user_groups g ON g.id=gu.group_id JOIN group_roles gr ON gr.group_id=g.id JOIN roles r ON r.id=gr.role_id WHERE gu.user_id=u.id AND g.tenant_id=$1 AND r.tenant_id=$1 AND r.id=ANY($3::bigint[]))
 ) ORDER BY u.id`, tenant, groups, roleIDs)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []uint{}
	for rows.Next() {
		var id uint
		if err := rows.Scan(&id); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result = append(result, id)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return result, nil
}
