package persistence

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationPushDeviceRepository struct{}

func NewNotificationPushDeviceRepository() notifications.PushDeviceRepository {
	return &NotificationPushDeviceRepository{}
}

// Register stores or refreshes a device. A token that another active
// registration of the tenant holds (a shared phone, a re-login) moves to this
// device, so one token never pushes to two accounts.
func (r *NotificationPushDeviceRepository) Register(ctx context.Context, d notifications.PushDevice) error {
	const op serrors.Op = "NotificationPushDeviceRepository.Register"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if d.UserID == 0 || d.DeviceID == "" || d.Token == "" || !d.Platform.Valid() {
		return serrors.New(serrors.Invalid, "Invalid push device").WithOp(op)
	}
	return composables.InTx(ctx, func(txCtx context.Context) error {
		db, err := composables.UseTx(txCtx)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		if _, err := db.Exec(txCtx, `UPDATE core.notification_push_devices SET revoked_at=NOW() WHERE tenant_id=$1 AND token=$2 AND revoked_at IS NULL AND NOT (user_id=$3 AND device_id=$4)`, tenant, d.Token, int(d.UserID), d.DeviceID); err != nil { //nolint:gosec // User IDs come from the users table.
			return serrors.Wrap(op, err)
		}
		_, err = db.Exec(txCtx, `INSERT INTO core.notification_push_devices(tenant_id,user_id,device_id,token,platform) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,user_id,device_id) DO UPDATE SET token=EXCLUDED.token,platform=EXCLUDED.platform,revoked_at=NULL`, tenant, int(d.UserID), d.DeviceID, d.Token, string(d.Platform)) //nolint:gosec // User IDs come from the users table.
		return serrors.Wrap(op, err)
	})
}

func (r *NotificationPushDeviceRepository) Revoke(ctx context.Context, userID uint, deviceID string) error {
	const op serrors.Op = "NotificationPushDeviceRepository.Revoke"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `UPDATE core.notification_push_devices SET revoked_at=NOW() WHERE tenant_id=$1 AND user_id=$2 AND device_id=$3 AND revoked_at IS NULL`, tenant, int(userID), deviceID) //nolint:gosec // User IDs come from the users table.
	return serrors.Wrap(op, err)
}

func (r *NotificationPushDeviceRepository) Active(ctx context.Context, userID uint) ([]notifications.PushDevice, error) {
	const op serrors.Op = "NotificationPushDeviceRepository.Active"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := db.Query(ctx, `SELECT tenant_id,user_id,device_id,token,platform,created_at,revoked_at FROM core.notification_push_devices WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL ORDER BY created_at,device_id`, tenant, int(userID)) //nolint:gosec // User IDs come from the users table.
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	result := []notifications.PushDevice{}
	for rows.Next() {
		var d notifications.PushDevice
		var user int
		var platform string
		if err := rows.Scan(&d.TenantID, &user, &d.DeviceID, &d.Token, &platform, &d.CreatedAt, &d.RevokedAt); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		d.UserID, d.Platform = uint(user), notifications.PushPlatform(platform) //nolint:gosec // Constrained by the users foreign key.
		result = append(result, d)
	}
	return result, serrors.Wrap(op, rows.Err())
}
