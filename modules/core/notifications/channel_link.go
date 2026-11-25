package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LinkTokenTTL is how long a link token can be used.
const LinkTokenTTL = 15 * time.Minute

// Revoked reasons of a link the provider ended.
const (
	// LinkRevokedTargetGone: the provider said the address is gone (the user
	// blocked the bot, the chat does not exist).
	LinkRevokedTargetGone = "target_gone"
)

// ChannelLink is a user's external account on a linked channel (Telegram now,
// any AddressLinked channel later).
type ChannelLink struct {
	TenantID uuid.UUID
	UserID   uint
	Channel  Channel
	// ConnectionID is the connection the link was made through: a connection
	// UUID or "platform:<kind>".
	ConnectionID string
	// Address is where the channel sends (telegram: the chat id).
	Address string
	// Display names the account for people (telegram: @username or the name).
	Display  string
	LinkedAt time.Time
}

// ChannelLinkState is what a user sees of a linked channel: the link, or why
// the last one ended.
type ChannelLinkState struct {
	// Link is nil when the user has no working link.
	Link *ChannelLink
	// Revoked is the reason the provider ended the last link ("" when none).
	Revoked   string
	RevokedAt *time.Time
}

// ChannelLinkRepository keeps the links of the tenant in ctx.
type ChannelLinkRepository interface {
	// Get returns the user's working link; ErrNoAddress when there is none.
	Get(ctx context.Context, userID uint, channel Channel) (ChannelLink, error)
	// State returns the user's link or the reason the last one ended.
	State(ctx context.Context, userID uint, channel Channel) (ChannelLinkState, error)
	// Save stores the one link of (tenant, user, channel). An address linked
	// to another user of the tenant through the same connection moves to this
	// user.
	Save(ctx context.Context, l ChannelLink) error
	// Delete removes the user's link.
	Delete(ctx context.Context, userID uint, channel Channel) error
	// DeleteByAddress ends the link of an address the provider reported gone;
	// the user keeps a revoked marker. An empty connectionID matches any.
	DeleteByAddress(ctx context.Context, channel Channel, connectionID string, address string) error
	// DeleteChannel removes every link of the channel in the tenant: the
	// addresses of one connection mean nothing to another.
	DeleteChannel(ctx context.Context, channel Channel) error
}

// LinkToken is a stored one-use link token. The token itself is never stored:
// Hash is its SHA-256.
type LinkToken struct {
	Hash         string
	TenantID     uuid.UUID
	UserID       uint
	Channel      Channel
	ConnectionID string
	Language     string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	UsedAt       *time.Time
	// UsedUpdateKey names the provider update that used the token.
	UsedUpdateKey string
	UsedAddress   string
	UsedDisplay   string
}

// LinkTokenRepository keeps link tokens. Tokens are global: Find and Use run
// without a tenant in ctx (the token is what names the tenant).
type LinkTokenRepository interface {
	Create(ctx context.Context, t LinkToken) error
	// Find returns the token by hash (ErrLinkTokenInvalid when unknown).
	Find(ctx context.Context, hash string) (LinkToken, error)
	// FindByUpdate returns the token an update used (ErrLinkTokenInvalid when none).
	FindByUpdate(ctx context.Context, updateKey string) (LinkToken, error)
	// Use marks an unused, unexpired token used by the update, in the caller's
	// transaction; false when it was used or expired meanwhile.
	Use(ctx context.Context, hash string, updateKey, address, display string, at time.Time) (bool, error)
	// Latest is the user's newest token of the channel (ErrLinkTokenInvalid when none).
	Latest(ctx context.Context, userID uint, channel Channel) (LinkToken, error)
	// Cancel expires the user's unused tokens of the channel.
	Cancel(ctx context.Context, userID uint, channel Channel) error
}

// LinkStart is a link the current user opens to confirm it.
type LinkStart struct {
	URL       string
	ExpiresAt time.Time
}

// LinkStatus is the current user's link of a channel, for the profile.
type LinkStatus struct {
	Channel Channel
	// Linked is set when the user has a working link.
	Linked  bool
	Display string
	// Revoked is why the last link ended, when no link replaced it.
	Revoked string
	// Pending is set while an unused, unexpired link token exists.
	Pending   bool
	ExpiresAt time.Time
	// Expired is set when the newest token expired unused.
	Expired bool
}

// ChannelLinkService links the current user's external accounts.
type ChannelLinkService interface {
	// StartLink issues a one-use link for the current user.
	StartLink(ctx context.Context, channel Channel) (LinkStart, error)
	// CompleteLink links the token's user: the tenant and the user come from
	// the token, connectionID is the connection the confirmation came through.
	CompleteLink(ctx context.Context, connectionID, token, address, display string) error
	// TenantOf is the tenant of a working token; false for an unknown, used or
	// expired one.
	TenantOf(ctx context.Context, token string) (uuid.UUID, bool, error)
	// Unlink removes the current user's link and cancels pending tokens.
	Unlink(ctx context.Context, channel Channel) error
}

// LinkCompletion is the outcome of a confirmation.
type LinkCompletion struct {
	TenantID uuid.UUID
	UserID   uint
	Language string
	Display  string
	// Replayed is set when the update had already used the token: nothing changed.
	Replayed bool
}

// UpdateLinker is the part of a ChannelLinkService a provider's confirmation
// handler uses: it completes a link once per provider update, so a redelivered
// update gets the answer of the first.
type UpdateLinker interface {
	ChannelLinkService
	// CompleteLinkUpdate is CompleteLink keyed by the provider's update.
	CompleteLinkUpdate(ctx context.Context, connectionID, token, address, display, updateKey string) (LinkCompletion, error)
	// TenantOfUpdate is the tenant of the token an update used.
	TenantOfUpdate(ctx context.Context, updateKey string) (uuid.UUID, bool, error)
	// LanguageOf is the language of the token's user ("" when unknown).
	LanguageOf(ctx context.Context, token string) string
}
