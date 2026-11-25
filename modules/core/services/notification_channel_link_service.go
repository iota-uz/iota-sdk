package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotificationChannelConnectionService picks and stores the connection a
// connector-backed channel sends through for a tenant. It implements
// notifications.ChannelConnections: the tenant's chosen connection while it
// works, else the platform credential enabled for the tenant.
type NotificationChannelConnectionService struct {
	repo       notifications.ChannelConnectionRepository
	links      notifications.ChannelLinkRepository
	registry   func() notifications.ChannelRegistry
	connectors func() notifications.ConnectorConnections
	pool       *pgxpool.Pool
}

// withPool gives ctx the database when the caller's context has none (a
// provider's webhook, a background sender).
func withPool(ctx context.Context, pool *pgxpool.Pool) context.Context {
	if pool == nil || ctx.Value(constants.PoolKey) != nil {
		return ctx
	}
	return composables.WithPool(ctx, pool)
}

// WithPool sets the database used when a caller's context carries none.
func (s *NotificationChannelConnectionService) WithPool(pool *pgxpool.Pool) *NotificationChannelConnectionService {
	s.pool = pool
	return s
}

var _ notifications.ChannelConnections = (*NotificationChannelConnectionService)(nil)

// NewNotificationChannelConnectionService reads the registry and the
// connector connections lazily: both come from other components.
func NewNotificationChannelConnectionService(repo notifications.ChannelConnectionRepository, links notifications.ChannelLinkRepository, registry func() notifications.ChannelRegistry, connectors func() notifications.ConnectorConnections) *NotificationChannelConnectionService {
	return &NotificationChannelConnectionService{repo: repo, links: links, registry: registry, connectors: connectors}
}

func (s *NotificationChannelConnectionService) list(ctx context.Context, tenantID uuid.UUID, spec notifications.ChannelSpec) ([]notifications.ConnectorConnection, error) {
	if spec.ConnectorKind == "" || s.connectors == nil {
		return nil, nil
	}
	connectors := s.connectors()
	if connectors == nil {
		return nil, nil
	}
	return connectors.Connections(ctx, tenantID, spec.ConnectorKind)
}

func hasConnection(list []notifications.ConnectorConnection, id string) bool {
	return slices.ContainsFunc(list, func(c notifications.ConnectorConnection) bool { return c.ID == id })
}

// Connection implements notifications.ChannelConnections.
func (s *NotificationChannelConnectionService) Connection(ctx context.Context, tenantID uuid.UUID, spec notifications.ChannelSpec) (string, bool, error) {
	const op serrors.Op = "NotificationChannelConnectionService.Connection"
	ctx = withPool(ctx, s.pool)
	if spec.ConnectorKind == "" {
		return "", false, nil
	}
	list, err := s.list(ctx, tenantID, spec)
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	chosen, ok, err := s.repo.Get(ctx, tenantID, spec.Channel)
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	if !ok {
		chosen = notifications.PlatformConnectionPrefix + spec.ConnectorKind
	}
	return chosen, hasConnection(list, chosen), nil
}

// ChannelConnectionOptions is the connection picker of one channel.
type ChannelConnectionOptions struct {
	Spec        notifications.ChannelSpec
	Connections []notifications.ConnectorConnection
	// Chosen is the stored choice ("" when the tenant chose none).
	Chosen string
	// Effective is the connection the channel sends through now; EffectiveOK
	// is false when it does not work (the channel is unavailable).
	Effective   string
	EffectiveOK bool
}

// Options lists the connector-backed channels with their connections, for the
// tenant in ctx.
func (s *NotificationChannelConnectionService) Options(ctx context.Context) ([]ChannelConnectionOptions, error) {
	const op serrors.Op = "NotificationChannelConnectionService.Options"
	ctx = withPool(ctx, s.pool)
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	registry := s.registry()
	if registry == nil {
		return nil, nil
	}
	all := registry.All()
	out := make([]ChannelConnectionOptions, 0, len(all))
	for _, spec := range all {
		if spec.ConnectorKind == "" {
			continue
		}
		if _, adapter, _ := registry.Lookup(spec.Channel); adapter == nil {
			continue
		}
		list, err := s.list(ctx, tenant, spec)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		chosen, ok, err := s.repo.Get(ctx, tenant, spec.Channel)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		option := ChannelConnectionOptions{Spec: spec, Connections: list}
		if ok {
			option.Chosen = chosen
			option.Effective = chosen
		} else {
			option.Effective = notifications.PlatformConnectionPrefix + spec.ConnectorKind
		}
		option.EffectiveOK = hasConnection(list, option.Effective)
		out = append(out, option)
	}
	return out, nil
}

// Set chooses the connection of a channel for the tenant in ctx ("" goes back
// to the platform credential). When the connection the channel sends through
// changes, every link of the channel is removed: an address of one bot means
// nothing to another. It reports whether links were removed.
func (s *NotificationChannelConnectionService) Set(ctx context.Context, channel notifications.Channel, connectionID string) (bool, error) {
	const op serrors.Op = "NotificationChannelConnectionService.Set"
	ctx = withPool(ctx, s.pool)
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	registry := s.registry()
	if registry == nil {
		return false, serrors.New(serrors.Invalid, "invalid notification channel").WithOp(op)
	}
	spec, adapter, ok := registry.Lookup(channel)
	if !ok || adapter == nil || spec.ConnectorKind == "" {
		return false, serrors.New(serrors.Invalid, "invalid notification channel").WithOp(op)
	}
	list, err := s.list(ctx, tenant, spec)
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	if connectionID != "" && !hasConnection(list, connectionID) {
		return false, serrors.New(serrors.Invalid, "the connection is not available").WithOp(op)
	}
	var by uint
	if u, err := composables.UseUser(ctx); err == nil && u != nil {
		by = u.ID()
	}
	next := connectionID
	if next == "" {
		next = notifications.PlatformConnectionPrefix + spec.ConnectorKind
	}
	removed := false
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		// Concurrent saves of the channel take turns, so each compares with the choice the one before stored.
		tx, err := composables.UseTx(txCtx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(txCtx, `SELECT pg_advisory_xact_lock(hashtextextended('core.notification_channel_connections:' || $1::text || ':' || $2, 0))`, tenant.String(), string(channel)); err != nil {
			return err
		}
		previous, chosen, err := s.repo.Get(txCtx, tenant, channel)
		if err != nil {
			return err
		}
		if !chosen {
			previous = notifications.PlatformConnectionPrefix + spec.ConnectorKind
		}
		if err := s.repo.Set(txCtx, tenant, channel, connectionID, by); err != nil {
			return err
		}
		if previous != next && spec.Address == notifications.AddressLinked && s.links != nil {
			removed = true
			return s.links.DeleteChannel(txCtx, channel)
		}
		return nil
	})
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	return removed, nil
}

// NotificationChannelLinkService links the current user's accounts on linked
// channels through one-use tokens. It implements notifications.UpdateLinker.
type NotificationChannelLinkService struct {
	registry func() notifications.ChannelRegistry
	conns    notifications.ChannelConnections
	links    notifications.ChannelLinkRepository
	tokens   notifications.LinkTokenRepository
	now      func() time.Time
	pool     *pgxpool.Pool
}

// WithPool sets the database used when a caller's context carries none (the
// provider's webhook).
func (s *NotificationChannelLinkService) WithPool(pool *pgxpool.Pool) *NotificationChannelLinkService {
	s.pool = pool
	return s
}

var _ notifications.UpdateLinker = (*NotificationChannelLinkService)(nil)

func NewNotificationChannelLinkService(registry func() notifications.ChannelRegistry, conns notifications.ChannelConnections, links notifications.ChannelLinkRepository, tokens notifications.LinkTokenRepository) *NotificationChannelLinkService {
	return &NotificationChannelLinkService{registry: registry, conns: conns, links: links, tokens: tokens, now: time.Now}
}

// WithClock replaces the clock (tests).
func (s *NotificationChannelLinkService) WithClock(now func() time.Time) *NotificationChannelLinkService {
	s.now = now
	return s
}

func hashLinkToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *NotificationChannelLinkService) linked(channel notifications.Channel) (notifications.ChannelSpec, error) {
	registry := s.registry()
	if registry == nil {
		return notifications.ChannelSpec{}, notifications.ErrChannelUnavailable
	}
	spec, adapter, ok := registry.Lookup(channel)
	if !ok || adapter == nil || spec.Address != notifications.AddressLinked || spec.LinkURL == nil {
		return notifications.ChannelSpec{}, notifications.ErrChannelUnavailable
	}
	return spec, nil
}

// StartLink implements notifications.ChannelLinkService.
func (s *NotificationChannelLinkService) StartLink(ctx context.Context, channel notifications.Channel) (notifications.LinkStart, error) {
	const op serrors.Op = "NotificationChannelLinkService.StartLink"
	ctx = withPool(ctx, s.pool)
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return notifications.LinkStart{}, serrors.New(serrors.PermissionDenied, "no current user").WithOp(op)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return notifications.LinkStart{}, serrors.Wrap(op, err)
	}
	spec, err := s.linked(channel)
	if err != nil {
		return notifications.LinkStart{}, serrors.Wrap(op, err)
	}
	connection, ok, err := s.conns.Connection(ctx, tenant, spec)
	if err != nil {
		return notifications.LinkStart{}, serrors.Wrap(op, err)
	}
	if !ok {
		return notifications.LinkStart{}, serrors.Wrap(op, notifications.ErrChannelUnavailable)
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return notifications.LinkStart{}, serrors.Wrap(op, err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	url, err := spec.LinkURL(ctx, tenant, connection, token)
	if err != nil {
		return notifications.LinkStart{}, serrors.Wrap(op, err)
	}
	now := s.now()
	t := notifications.LinkToken{Hash: hashLinkToken(token), TenantID: tenant, UserID: u.ID(), Channel: channel, ConnectionID: connection,
		Language: string(u.UILanguage()), CreatedAt: now, ExpiresAt: now.Add(notifications.LinkTokenTTL)}
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		// One pending link at a time: a new one replaces the one the user did not finish.
		if err := s.tokens.Cancel(txCtx, u.ID(), channel); err != nil {
			return err
		}
		return s.tokens.Create(txCtx, t)
	})
	if err != nil {
		return notifications.LinkStart{}, serrors.Wrap(op, err)
	}
	return notifications.LinkStart{URL: url, ExpiresAt: t.ExpiresAt}, nil
}

// CompleteLink implements notifications.ChannelLinkService.
func (s *NotificationChannelLinkService) CompleteLink(ctx context.Context, connectionID, token, address, display string) error {
	_, err := s.CompleteLinkUpdate(ctx, connectionID, token, address, display, "")
	return err
}

// CompleteLinkUpdate links the user of the token to address. The token must be
// unused, unexpired and issued for connectionID, and the connection must still
// be the one the tenant's channel sends through. The same update completing
// the same token again gets the first result without changing anything.
func (s *NotificationChannelLinkService) CompleteLinkUpdate(ctx context.Context, connectionID, token, address, display, updateKey string) (notifications.LinkCompletion, error) {
	const op serrors.Op = "NotificationChannelLinkService.CompleteLinkUpdate"
	ctx = withPool(ctx, s.pool)
	if token == "" || address == "" || connectionID == "" {
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	hash := hashLinkToken(token)
	t, err := s.tokens.Find(ctx, hash)
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		return notifications.LinkCompletion{}, err
	}
	if err != nil {
		return notifications.LinkCompletion{}, serrors.Wrap(op, err)
	}
	done := notifications.LinkCompletion{TenantID: t.TenantID, UserID: t.UserID, Language: t.Language, Display: display}
	if t.UsedAt != nil {
		if updateKey != "" && t.UsedUpdateKey == updateKey && t.ConnectionID == connectionID {
			done.Replayed, done.Display = true, t.UsedDisplay
			return done, nil
		}
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	now := s.now()
	if !now.Before(t.ExpiresAt) || t.ConnectionID != connectionID {
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	tctx := composables.WithTenantID(ctx, t.TenantID)
	spec, err := s.linked(t.Channel)
	if err != nil {
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	current, ok, err := s.conns.Connection(tctx, t.TenantID, spec)
	if err != nil {
		return notifications.LinkCompletion{}, serrors.Wrap(op, err)
	}
	if !ok || current != connectionID {
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	err = composables.InTx(tctx, func(txCtx context.Context) error {
		// Expiry is checked again when the token is spent: the connection lookup above may have taken time.
		used, err := s.tokens.Use(txCtx, hash, updateKey, address, display, s.now())
		if err != nil {
			return err
		}
		if !used {
			return notifications.ErrLinkTokenInvalid
		}
		return s.links.Save(txCtx, notifications.ChannelLink{TenantID: t.TenantID, UserID: t.UserID, Channel: t.Channel, ConnectionID: connectionID, Address: address, Display: display, LinkedAt: now})
	})
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		// A concurrent delivery of the same update may have spent the token first: it gets that result.
		if updateKey != "" {
			if spent, ferr := s.tokens.Find(ctx, hash); ferr == nil && spent.UsedAt != nil && spent.UsedUpdateKey == updateKey && spent.ConnectionID == connectionID {
				done.Replayed, done.Display = true, spent.UsedDisplay
				return done, nil
			}
		}
		return notifications.LinkCompletion{}, err
	}
	if err != nil {
		return notifications.LinkCompletion{}, serrors.Wrap(op, err)
	}
	return done, nil
}

// TenantOf implements notifications.ChannelLinkService.
func (s *NotificationChannelLinkService) TenantOf(ctx context.Context, token string) (uuid.UUID, bool, error) {
	const op serrors.Op = "NotificationChannelLinkService.TenantOf"
	ctx = withPool(ctx, s.pool)
	if token == "" {
		return uuid.Nil, false, nil
	}
	t, err := s.tokens.Find(ctx, hashLinkToken(token))
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, serrors.Wrap(op, err)
	}
	if t.UsedAt != nil || !s.now().Before(t.ExpiresAt) {
		return uuid.Nil, false, nil
	}
	return t.TenantID, true, nil
}

// TenantOfUpdate implements notifications.UpdateLinker.
func (s *NotificationChannelLinkService) TenantOfUpdate(ctx context.Context, updateKey string) (uuid.UUID, bool, error) {
	const op serrors.Op = "NotificationChannelLinkService.TenantOfUpdate"
	ctx = withPool(ctx, s.pool)
	t, err := s.tokens.FindByUpdate(ctx, updateKey)
	if errors.Is(err, notifications.ErrLinkTokenInvalid) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, serrors.Wrap(op, err)
	}
	return t.TenantID, true, nil
}

// LanguageOf implements notifications.UpdateLinker.
func (s *NotificationChannelLinkService) LanguageOf(ctx context.Context, token string) string {
	ctx = withPool(ctx, s.pool)
	if token == "" {
		return ""
	}
	t, err := s.tokens.Find(ctx, hashLinkToken(token))
	if err != nil {
		return ""
	}
	return t.Language
}

// Unlink implements notifications.ChannelLinkService.
func (s *NotificationChannelLinkService) Unlink(ctx context.Context, channel notifications.Channel) error {
	const op serrors.Op = "NotificationChannelLinkService.Unlink"
	ctx = withPool(ctx, s.pool)
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return serrors.New(serrors.PermissionDenied, "no current user").WithOp(op)
	}
	return serrors.Wrap(op, composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.links.Delete(txCtx, u.ID(), channel); err != nil {
			return err
		}
		return s.tokens.Cancel(txCtx, u.ID(), channel)
	}))
}

// CancelLink drops the current user's unfinished link.
func (s *NotificationChannelLinkService) CancelLink(ctx context.Context, channel notifications.Channel) error {
	const op serrors.Op = "NotificationChannelLinkService.CancelLink"
	ctx = withPool(ctx, s.pool)
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return serrors.New(serrors.PermissionDenied, "no current user").WithOp(op)
	}
	return serrors.Wrap(op, s.tokens.Cancel(ctx, u.ID(), channel))
}

// Status is the current user's link of a channel.
func (s *NotificationChannelLinkService) Status(ctx context.Context, channel notifications.Channel) (notifications.LinkStatus, error) {
	const op serrors.Op = "NotificationChannelLinkService.Status"
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return notifications.LinkStatus{}, serrors.New(serrors.PermissionDenied, "no current user").WithOp(op)
	}
	return s.StatusOf(ctx, u.ID(), channel)
}

// StatusOf is a user's link of a channel in the tenant of ctx.
func (s *NotificationChannelLinkService) StatusOf(ctx context.Context, userID uint, channel notifications.Channel) (notifications.LinkStatus, error) {
	const op serrors.Op = "NotificationChannelLinkService.StatusOf"
	ctx = withPool(ctx, s.pool)
	status := notifications.LinkStatus{Channel: channel}
	state, err := s.links.State(ctx, userID, channel)
	if err != nil {
		return status, serrors.Wrap(op, err)
	}
	if state.Link != nil {
		status.Linked, status.Display = true, state.Link.Display
		return status, nil
	}
	status.Revoked = state.Revoked
	latest, err := s.tokens.Latest(ctx, userID, channel)
	switch {
	case errors.Is(err, notifications.ErrLinkTokenInvalid):
		return status, nil
	case err != nil:
		return status, serrors.Wrap(op, err)
	}
	if latest.UsedAt != nil || (state.RevokedAt != nil && latest.CreatedAt.Before(*state.RevokedAt)) {
		return status, nil
	}
	if s.now().Before(latest.ExpiresAt) {
		status.Pending, status.ExpiresAt = true, latest.ExpiresAt
	} else {
		status.Expired = true
	}
	return status, nil
}
