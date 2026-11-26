package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotificationChannelLinkRepository keeps core.notification_channel_links.
type NotificationChannelLinkRepository struct{}

func NewNotificationChannelLinkRepository() notifications.ChannelLinkRepository {
	return &NotificationChannelLinkRepository{}
}

func linkScope(ctx context.Context) (uuid.UUID, error) {
	return composables.UseTenantID(ctx)
}

func (r *NotificationChannelLinkRepository) Get(ctx context.Context, userID uint, channel notifications.Channel) (notifications.ChannelLink, error) {
	const op serrors.Op = "NotificationChannelLinkRepository.Get"
	state, err := r.State(ctx, userID, channel)
	if err != nil {
		return notifications.ChannelLink{}, serrors.Wrap(op, err)
	}
	if state.Link == nil {
		return notifications.ChannelLink{}, notifications.ErrNoAddress
	}
	return *state.Link, nil
}

func (r *NotificationChannelLinkRepository) State(ctx context.Context, userID uint, channel notifications.Channel) (notifications.ChannelLinkState, error) {
	const op serrors.Op = "NotificationChannelLinkRepository.State"
	tenant, err := linkScope(ctx)
	if err != nil {
		return notifications.ChannelLinkState{}, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return notifications.ChannelLinkState{}, serrors.Wrap(op, err)
	}
	var (
		link      notifications.ChannelLink
		revoked   string
		revokedAt *time.Time
	)
	err = db.QueryRow(ctx, `SELECT connection_id,address,display,linked_at,revoked_reason,revoked_at FROM core.notification_channel_links WHERE tenant_id=$1 AND user_id=$2 AND channel=$3`,
		tenant, int(userID), string(channel)). //nolint:gosec // User IDs are serial integers.
		Scan(&link.ConnectionID, &link.Address, &link.Display, &link.LinkedAt, &revoked, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return notifications.ChannelLinkState{}, nil
	}
	if err != nil {
		return notifications.ChannelLinkState{}, serrors.Wrap(op, err)
	}
	if revoked != "" {
		return notifications.ChannelLinkState{Revoked: revoked, RevokedAt: revokedAt}, nil
	}
	link.TenantID, link.UserID, link.Channel = tenant, userID, channel
	return notifications.ChannelLinkState{Link: &link}, nil
}

func (r *NotificationChannelLinkRepository) Save(ctx context.Context, l notifications.ChannelLink) error {
	const op serrors.Op = "NotificationChannelLinkRepository.Save"
	tenant, err := linkScope(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if l.TenantID != uuid.Nil && l.TenantID != tenant {
		return serrors.New(serrors.Invalid, "link of another tenant").WithOp(op)
	}
	if l.UserID == 0 || !l.Channel.Valid() || l.ConnectionID == "" || l.Address == "" {
		return serrors.New(serrors.Invalid, "invalid channel link").WithOp(op)
	}
	if l.LinkedAt.IsZero() {
		l.LinkedAt = time.Now()
	}
	return composables.InTx(ctx, func(txCtx context.Context) error {
		db, err := composables.UseTx(txCtx)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		// The address moves: a chat belongs to one person of the tenant per connection.
		if _, err := db.Exec(txCtx, `DELETE FROM core.notification_channel_links WHERE tenant_id=$1 AND channel=$2 AND connection_id=$3 AND address=$4 AND user_id<>$5`,
			tenant, string(l.Channel), l.ConnectionID, l.Address, int(l.UserID)); err != nil { //nolint:gosec // User IDs are serial integers.
			return serrors.Wrap(op, err)
		}
		_, err = db.Exec(txCtx, `INSERT INTO core.notification_channel_links(tenant_id,user_id,channel,connection_id,address,display,linked_at)
			VALUES($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (tenant_id,user_id,channel) DO UPDATE SET connection_id=EXCLUDED.connection_id,address=EXCLUDED.address,
			display=EXCLUDED.display,linked_at=EXCLUDED.linked_at,revoked_reason='',revoked_at=NULL`,
			tenant, int(l.UserID), string(l.Channel), l.ConnectionID, l.Address, truncateRunes(l.Display, 200), l.LinkedAt) //nolint:gosec // User IDs are serial integers.
		return serrors.Wrap(op, err)
	})
}

func (r *NotificationChannelLinkRepository) Delete(ctx context.Context, userID uint, channel notifications.Channel) error {
	const op serrors.Op = "NotificationChannelLinkRepository.Delete"
	tenant, err := linkScope(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `DELETE FROM core.notification_channel_links WHERE tenant_id=$1 AND user_id=$2 AND channel=$3`, tenant, int(userID), string(channel)) //nolint:gosec // User IDs are serial integers.
	return serrors.Wrap(op, err)
}

func (r *NotificationChannelLinkRepository) DeleteByAddress(ctx context.Context, channel notifications.Channel, connectionID string, address string) error {
	const op serrors.Op = "NotificationChannelLinkRepository.DeleteByAddress"
	tenant, err := linkScope(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `UPDATE core.notification_channel_links SET revoked_reason=$5, revoked_at=NOW()
		WHERE tenant_id=$1 AND channel=$2 AND ($3='' OR connection_id=$3) AND address=$4 AND revoked_reason=''`,
		tenant, string(channel), connectionID, address, notifications.LinkRevokedTargetGone)
	return serrors.Wrap(op, err)
}

func (r *NotificationChannelLinkRepository) DeleteChannel(ctx context.Context, channel notifications.Channel) error {
	const op serrors.Op = "NotificationChannelLinkRepository.DeleteChannel"
	tenant, err := linkScope(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM core.notification_channel_links WHERE tenant_id=$1 AND channel=$2`, tenant, string(channel)); err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `UPDATE core.notification_channel_link_tokens SET expires_at=LEAST(expires_at,NOW()) WHERE tenant_id=$1 AND channel=$2 AND used_at IS NULL`, tenant, string(channel))
	return serrors.Wrap(op, err)
}

// NotificationLinkTokenRepository keeps core.notification_channel_link_tokens.
// Tokens are global: Find, FindByUpdate and Use read without a tenant.
type NotificationLinkTokenRepository struct{}

func NewNotificationLinkTokenRepository() notifications.LinkTokenRepository {
	return &NotificationLinkTokenRepository{}
}

const linkTokenColumns = `token_hash,tenant_id,user_id,channel,connection_id,language,created_at,expires_at,used_at,used_update_key,used_address,used_display`

func scanLinkToken(row pgx.Row) (notifications.LinkToken, error) {
	var (
		t       notifications.LinkToken
		userID  int
		channel string
	)
	if err := row.Scan(&t.Hash, &t.TenantID, &userID, &channel, &t.ConnectionID, &t.Language, &t.CreatedAt, &t.ExpiresAt, &t.UsedAt, &t.UsedUpdateKey, &t.UsedAddress, &t.UsedDisplay); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return t, notifications.ErrLinkTokenInvalid
		}
		return t, err
	}
	t.UserID, t.Channel = uint(userID), notifications.Channel(channel) //nolint:gosec // Constrained by the users foreign key.
	return t, nil
}

func (r *NotificationLinkTokenRepository) Create(ctx context.Context, t notifications.LinkToken) error {
	const op serrors.Op = "NotificationLinkTokenRepository.Create"
	if t.TenantID == uuid.Nil || t.UserID == 0 || !t.Channel.Valid() || t.ConnectionID == "" || t.ExpiresAt.IsZero() {
		return serrors.New(serrors.Invalid, "invalid link token").WithOp(op)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = db.Exec(ctx, `INSERT INTO core.notification_channel_link_tokens(token_hash,tenant_id,user_id,channel,connection_id,language,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		t.Hash, t.TenantID, int(t.UserID), string(t.Channel), t.ConnectionID, t.Language, t.ExpiresAt) //nolint:gosec // User IDs are serial integers.
	return serrors.Wrap(op, err)
}

func (r *NotificationLinkTokenRepository) Find(ctx context.Context, hash string) (notifications.LinkToken, error) {
	const op serrors.Op = "NotificationLinkTokenRepository.Find"
	db, err := composables.UseTx(ctx)
	if err != nil {
		return notifications.LinkToken{}, serrors.Wrap(op, err)
	}
	t, err := scanLinkToken(db.QueryRow(ctx, `SELECT `+linkTokenColumns+` FROM core.notification_channel_link_tokens WHERE token_hash=$1`, hash))
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		return t, err
	}
	return t, serrors.Wrap(op, err)
}

func (r *NotificationLinkTokenRepository) FindByUpdate(ctx context.Context, updateKey string) (notifications.LinkToken, error) {
	const op serrors.Op = "NotificationLinkTokenRepository.FindByUpdate"
	if updateKey == "" {
		return notifications.LinkToken{}, notifications.ErrLinkTokenInvalid
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return notifications.LinkToken{}, serrors.Wrap(op, err)
	}
	t, err := scanLinkToken(db.QueryRow(ctx, `SELECT `+linkTokenColumns+` FROM core.notification_channel_link_tokens WHERE used_update_key=$1 ORDER BY used_at DESC LIMIT 1`, updateKey))
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		return t, err
	}
	return t, serrors.Wrap(op, err)
}

func (r *NotificationLinkTokenRepository) Use(ctx context.Context, hash string, updateKey, address, display string, at time.Time) (bool, error) {
	const op serrors.Op = "NotificationLinkTokenRepository.Use"
	db, err := composables.UseTx(ctx)
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	tag, err := db.Exec(ctx, `UPDATE core.notification_channel_link_tokens SET used_at=$2,used_update_key=$3,used_address=$4,used_display=$5
		WHERE token_hash=$1 AND used_at IS NULL AND expires_at>$2 AND expires_at>clock_timestamp()`, hash, at, updateKey, address, truncateRunes(display, 200))
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *NotificationLinkTokenRepository) Latest(ctx context.Context, userID uint, channel notifications.Channel) (notifications.LinkToken, error) {
	const op serrors.Op = "NotificationLinkTokenRepository.Latest"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return notifications.LinkToken{}, serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return notifications.LinkToken{}, serrors.Wrap(op, err)
	}
	t, err := scanLinkToken(db.QueryRow(ctx, `SELECT `+linkTokenColumns+` FROM core.notification_channel_link_tokens WHERE tenant_id=$1 AND user_id=$2 AND channel=$3 ORDER BY created_at DESC LIMIT 1`,
		tenant, int(userID), string(channel))) //nolint:gosec // User IDs are serial integers.
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		return t, err
	}
	return t, serrors.Wrap(op, err)
}

func (r *NotificationLinkTokenRepository) Cancel(ctx context.Context, userID uint, channel notifications.Channel) error {
	const op serrors.Op = "NotificationLinkTokenRepository.Cancel"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	// A cancelled token is no longer pending, and reads as cancelled rather than expired.
	_, err = db.Exec(ctx, `DELETE FROM core.notification_channel_link_tokens WHERE tenant_id=$1 AND user_id=$2 AND channel=$3 AND used_at IS NULL`,
		tenant, int(userID), string(channel)) //nolint:gosec // User IDs are serial integers.
	return serrors.Wrap(op, err)
}

// NotificationChannelConnectionRepository keeps core.notification_channel_connections.
type NotificationChannelConnectionRepository struct{}

func NewNotificationChannelConnectionRepository() notifications.ChannelConnectionRepository {
	return &NotificationChannelConnectionRepository{}
}

func (r *NotificationChannelConnectionRepository) Get(ctx context.Context, tenantID uuid.UUID, channel notifications.Channel) (string, bool, error) {
	const op serrors.Op = "NotificationChannelConnectionRepository.Get"
	db, err := composables.UseTx(ctx)
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	var id string
	err = db.QueryRow(ctx, `SELECT connection_id FROM core.notification_channel_connections WHERE tenant_id=$1 AND channel=$2`, tenantID, string(channel)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	return id, true, nil
}

func (r *NotificationChannelConnectionRepository) Set(ctx context.Context, tenantID uuid.UUID, channel notifications.Channel, connectionID string, by uint) error {
	const op serrors.Op = "NotificationChannelConnectionRepository.Set"
	if !channel.Valid() {
		return serrors.New(serrors.Invalid, "invalid notification channel").WithOp(op)
	}
	db, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if connectionID == "" {
		_, err = db.Exec(ctx, `DELETE FROM core.notification_channel_connections WHERE tenant_id=$1 AND channel=$2`, tenantID, string(channel))
		return serrors.Wrap(op, err)
	}
	var updatedBy *int
	if by != 0 {
		v := int(by) //nolint:gosec // User IDs are serial integers.
		updatedBy = &v
	}
	_, err = db.Exec(ctx, `INSERT INTO core.notification_channel_connections(tenant_id,channel,connection_id,updated_by) VALUES($1,$2,$3,$4)
		ON CONFLICT (tenant_id,channel) DO UPDATE SET connection_id=EXCLUDED.connection_id,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
		tenantID, string(channel), connectionID, updatedBy)
	return serrors.Wrap(op, err)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
