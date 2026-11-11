package persistence

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationPreferenceRepository struct{}

func NewNotificationPreferenceRepository() notifications.PreferenceRepository {
	return &NotificationPreferenceRepository{}
}

func (r *NotificationPreferenceRepository) Disabled(ctx context.Context, userIDs []uint) (map[uint][]notifications.Channel, error) {
	const op serrors.Op = "NotificationPreferenceRepository.Disabled"
	result := map[uint][]notifications.Channel{}
	if len(userIDs) == 0 {
		return result, nil
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	ids := make([]int32, 0, len(userIDs))
	for _, id := range userIDs {
		ids = append(ids, int32(id)) //nolint:gosec // User IDs are serial integers.
	}
	rows, err := db.Query(ctx, `SELECT user_id,channel FROM core.notification_channel_preferences WHERE tenant_id=$1 AND user_id=ANY($2) ORDER BY user_id,channel`, tenant, ids)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var channel string
		if err := rows.Scan(&id, &channel); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		result[uint(id)] = append(result[uint(id)], notifications.Channel(channel)) //nolint:gosec // Constrained by the users foreign key.
	}
	return result, serrors.Wrap(op, rows.Err())
}

func (r *NotificationPreferenceRepository) SetDisabled(ctx context.Context, userID uint, channels []notifications.Channel) error {
	const op serrors.Op = "NotificationPreferenceRepository.SetDisabled"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM core.notification_channel_preferences WHERE tenant_id=$1 AND user_id=$2`, tenant, int(userID)); err != nil { //nolint:gosec // User IDs are serial integers.
		return serrors.Wrap(op, err)
	}
	for _, c := range notifications.UniqueChannels(channels) {
		if !c.Valid() {
			return serrors.New(serrors.Invalid, "invalid notification channel").WithOp(op)
		}
		if _, err := db.Exec(ctx, `INSERT INTO core.notification_channel_preferences(tenant_id,user_id,channel) VALUES($1,$2,$3)`, tenant, int(userID), string(c)); err != nil { //nolint:gosec // User IDs are serial integers.
			return serrors.Wrap(op, err)
		}
	}
	return nil
}
