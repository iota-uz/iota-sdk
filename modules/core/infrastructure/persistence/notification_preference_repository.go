package persistence

import (
	"context"
	"slices"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotificationPreferenceRepository keeps core.notification_channel_preferences.
// A row of an ordinary channel means the user turned it off; a row of an
// opt-in channel means the user turned it on.
type NotificationPreferenceRepository struct {
	optIn func() []notifications.Channel
}

// PreferenceOption configures the preference repository.
type PreferenceOption func(*NotificationPreferenceRepository)

// WithOptInChannels names the opt-in channels (the registry's, read per call).
func WithOptInChannels(optIn func() []notifications.Channel) PreferenceOption {
	return func(r *NotificationPreferenceRepository) { r.optIn = optIn }
}

func NewNotificationPreferenceRepository(opts ...PreferenceOption) notifications.PreferenceRepository {
	r := &NotificationPreferenceRepository{}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *NotificationPreferenceRepository) optInChannels() []notifications.Channel {
	if r.optIn == nil {
		return nil
	}
	return r.optIn()
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
	optIn := r.optInChannels()
	enabledOptIn := map[uint][]notifications.Channel{}
	for rows.Next() {
		var id int
		var channel string
		if err := rows.Scan(&id, &channel); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		uid, c := uint(id), notifications.Channel(channel) //nolint:gosec // Constrained by the users foreign key.
		if slices.Contains(optIn, c) {
			enabledOptIn[uid] = append(enabledOptIn[uid], c)
			continue
		}
		result[uid] = append(result[uid], c)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if len(optIn) > 0 {
		for _, id := range userIDs {
			for _, c := range optIn {
				if !slices.Contains(enabledOptIn[id], c) {
					result[id] = append(result[id], c)
				}
			}
		}
	}
	return result, nil
}

func (r *NotificationPreferenceRepository) SetDisabled(ctx context.Context, userID uint, channels []notifications.Channel) error {
	const op serrors.Op = "NotificationPreferenceRepository.SetDisabled"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	disabled := notifications.UniqueChannels(channels)
	for _, c := range disabled {
		if !c.Valid() {
			return serrors.New(serrors.Invalid, "invalid notification channel").WithOp(op)
		}
	}
	optIn := r.optInChannels()
	var rowsToStore []notifications.Channel
	for _, c := range disabled {
		if !slices.Contains(optIn, c) {
			rowsToStore = append(rowsToStore, c)
		}
	}
	for _, c := range optIn {
		if !slices.Contains(disabled, c) {
			rowsToStore = append(rowsToStore, c)
		}
	}
	return composables.InTx(ctx, func(txCtx context.Context) error {
		db, err := composables.UseTx(txCtx)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		if _, err := db.Exec(txCtx, `DELETE FROM core.notification_channel_preferences WHERE tenant_id=$1 AND user_id=$2`, tenant, int(userID)); err != nil { //nolint:gosec // User IDs are serial integers.
			return serrors.Wrap(op, err)
		}
		for _, c := range rowsToStore {
			if _, err := db.Exec(txCtx, `INSERT INTO core.notification_channel_preferences(tenant_id,user_id,channel) VALUES($1,$2,$3)`, tenant, int(userID), string(c)); err != nil { //nolint:gosec // User IDs are serial integers.
				return serrors.Wrap(op, err)
			}
		}
		return nil
	})
}
