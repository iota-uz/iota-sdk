package notifications

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
)

// Channel is a delivery medium of the shared notification system.
type Channel string

const (
	ChannelInApp Channel = "in_app"
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelPush  Channel = "push"
)

// Channels lists every known channel in display order.
func Channels() []Channel {
	return []Channel{ChannelInApp, ChannelEmail, ChannelSMS, ChannelPush}
}

// Valid reports whether c is a known channel.
func (c Channel) Valid() bool {
	switch c {
	case ChannelInApp, ChannelEmail, ChannelSMS, ChannelPush:
		return true
	}
	return false
}

// External reports whether delivery leaves the SDK through a provider adapter.
func (c Channel) External() bool { return c.Valid() && c != ChannelInApp }

// NormalizeChannels deduplicates channels preserving order; an empty selection
// means in-app only, the behaviour of rules saved before channels existed.
func NormalizeChannels(in []Channel) []Channel {
	out := make([]Channel, 0, len(in))
	seen := map[Channel]bool{}
	for _, c := range in {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return []Channel{ChannelInApp}
	}
	return out
}

// DeliveryStatus is the outcome of one recipient on one channel.
type DeliveryStatus string

const (
	// DeliveryPending is waiting for its first or next provider attempt.
	DeliveryPending DeliveryStatus = "pending"
	// DeliverySent was accepted by the provider (or stored, for in-app).
	DeliverySent DeliveryStatus = "sent"
	// DeliveryFailed exhausted its attempts or hit a permanent provider error.
	DeliveryFailed DeliveryStatus = "failed"
	// DeliveryNoAddress means the recipient has no address/device for the channel.
	DeliveryNoAddress DeliveryStatus = "no_address"
)

// MaxDeliveryAttempts bounds automatic provider attempts for one delivery.
const MaxDeliveryAttempts = 5

var (
	// ErrNoAddress is returned by an AddressResolver when the recipient has no
	// address or device for the channel.
	ErrNoAddress = errors.New("notification recipient has no address for channel")
	// ErrPermanent marks a provider error that must not be retried.
	ErrPermanent = errors.New("permanent notification provider failure")
	// ErrDeliveryNotFound is returned when a delivery does not exist in the tenant.
	ErrDeliveryNotFound = errors.New("notification delivery not found")
)

// Message is the provider-neutral payload handed to a ChannelAdapter. It holds
// no credentials; adapters own their provider configuration.
type Message struct {
	DeliveryID     uuid.UUID
	TenantID       uuid.UUID
	EventKey       string
	EventID        string
	ContentVersion int
	UserID         uint
	Channel        Channel
	Address        string
	Language       string
	Level          notification.Level
	Title          string
	Body           string
	ActionURL      string
}

// Receipt is what a provider returned for an accepted message.
type Receipt struct{ ProviderMessageID string }

// ChannelAdapter sends one message through a provider. Return an error wrapping
// ErrPermanent when retrying cannot succeed. Adapters must be safe for concurrent
// use and must not be called inside a database transaction.
type ChannelAdapter interface {
	Channel() Channel
	Send(context.Context, Message) (Receipt, error)
}

// AddressResolver finds where a recipient is reachable on a channel: an email
// address, phone number or push device token. Return ErrNoAddress when none.
type AddressResolver interface {
	Resolve(ctx context.Context, tenantID uuid.UUID, userID uint, channel Channel) (string, error)
}

// Delivery is the persisted state of one event/recipient/channel.
type Delivery struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	EventKey        string
	EventID         string
	ContentVersion  int
	UserID          uint
	Channel         Channel
	Status          DeliveryStatus
	Language        string
	Level           notification.Level
	Title           string
	Body            string
	ActionURL       string
	Attempts        int
	LastError       string
	ProviderReceipt string
	CreatedAt       time.Time
	AvailableAt     time.Time
	DeliveredAt     *time.Time
}

// DeliveryStats counts a tenant's deliveries per channel and status.
type DeliveryStats struct {
	Channel Channel
	Status  DeliveryStatus
	Count   int64
}

// DeliveryRepository persists per-channel delivery state. Create runs inside the
// caller's transaction; Claim and Record never hold a transaction across a
// provider call.
type DeliveryRepository interface {
	// Create stores deliveries idempotently by event, recipient and channel.
	Create(context.Context, []Delivery) error
	// Claim leases up to limit due external deliveries and counts the attempt.
	Claim(ctx context.Context, limit int, lease time.Duration) ([]Delivery, error)
	// Record stores the outcome of an attempt.
	Record(ctx context.Context, id uuid.UUID, status DeliveryStatus, receipt, lastError string, retryIn time.Duration) error
	// Retry makes one failed or exhausted delivery due again, leaving siblings alone.
	Retry(context.Context, uuid.UUID) error
	// ForEvent lists the deliveries of one event.
	ForEvent(ctx context.Context, eventKey, eventID string) ([]Delivery, error)
	Stats(context.Context) ([]DeliveryStats, error)
}
