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

// UniqueChannels deduplicates channels preserving order, without the in-app default.
func UniqueChannels(in []Channel) []Channel {
	out := make([]Channel, 0, len(in))
	seen := map[Channel]bool{}
	for _, c := range in {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

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

// DeliveryStatus is the state of one delivery: one notice, one recipient, one
// channel and one concrete target.
type DeliveryStatus string

const (
	// DeliveryUnresolved is a recipient/channel waiting for the worker to
	// expand it into targets, apply policy and render its content.
	DeliveryUnresolved DeliveryStatus = "unresolved"
	// DeliveryPending is waiting for its first or next provider attempt.
	DeliveryPending DeliveryStatus = "pending"
	// DeliverySent was accepted by the provider (or stored, for in-app).
	DeliverySent DeliveryStatus = "sent"
	// DeliveryFailed exhausted its attempts or hit a permanent provider error.
	DeliveryFailed DeliveryStatus = "failed"
	// DeliveryAmbiguous means the provider may or may not have accepted the
	// message (timeout, lost reply, crash). It is never retried automatically
	// because a retry could duplicate the message; an operator retries it.
	DeliveryAmbiguous DeliveryStatus = "ambiguous"
	// DeliveryNoAddress means the recipient has no address/device for the channel.
	DeliveryNoAddress DeliveryStatus = "no_address"
	// DeliverySkipped means access or business policy stopped the delivery;
	// the error code says which.
	DeliverySkipped DeliveryStatus = "skipped"
)

// Machine-readable delivery error codes. They never carry provider text.
const (
	ErrorCodeNoAddress            = "no_address"
	ErrorCodeContactNotFound      = "contact_not_found"
	ErrorCodeSubjectAccessDenied  = "subject_access_denied"
	ErrorCodePolicyDenied         = "policy_denied"
	ErrorCodeAdapterMissing       = "adapter_not_installed"
	ErrorCodeProviderFailure      = "provider_failure"
	ErrorCodeProviderRejected     = "provider_rejected"
	ErrorCodeProviderAmbiguous    = "provider_ambiguous"
	ErrorCodeAttemptLimit         = "attempt_limit"
	ErrorCodeLeaseExpired         = "lease_expired"
	ErrorCodeResolutionFailed     = "resolution_failed"
	ErrorCodeResolutionLimit      = "resolution_attempt_limit"
	ErrorCodeResolverMissing      = "resolution_resolver_missing"
	ErrorCodeResolutionRender     = "resolution_render_failed"
	ErrorCodeResolutionBadContent = "resolution_invalid_content"
)

// AttemptOutcome is the result of one provider call.
type AttemptOutcome string

const (
	// AttemptSent was accepted by the provider.
	AttemptSent AttemptOutcome = "sent"
	// AttemptFailed failed in a way that may succeed on retry.
	AttemptFailed AttemptOutcome = "failed"
	// AttemptPermanent was rejected and must not be retried.
	AttemptPermanent AttemptOutcome = "permanent"
	// AttemptAmbiguous is an unknown outcome: the provider may have accepted it.
	AttemptAmbiguous AttemptOutcome = "ambiguous"
)

// MaxDeliveryAttempts bounds automatic attempts for one delivery per retry budget.
const MaxDeliveryAttempts = 5

var (
	// ErrNoAddress is returned by an AddressResolver when the recipient has no
	// address or device for the channel.
	ErrNoAddress = errors.New("notification recipient has no address for channel")
	// ErrPermanent marks a provider error that must not be retried.
	ErrPermanent = errors.New("permanent notification provider failure")
	// ErrAmbiguous marks a provider error whose outcome is unknown: the message
	// may have been accepted (timeout after the request was sent, lost reply).
	ErrAmbiguous = errors.New("ambiguous notification provider outcome")
	// ErrDeliveryNotFound is returned when a delivery does not exist in the tenant.
	ErrDeliveryNotFound = errors.New("notification delivery not found")
)

// Message is the provider-neutral payload handed to a ChannelAdapter. It holds
// no credentials; adapters own their provider configuration.
type Message struct {
	DeliveryID uuid.UUID
	// IdempotencyKey is stable across the attempts of one delivery. Providers
	// that support idempotent sends should use it so a retried or ambiguous
	// attempt does not duplicate the message.
	IdempotencyKey string
	Attempt        int
	TenantID       uuid.UUID
	NoticeID       uuid.UUID
	EventKey       string
	EventID        string
	SnapshotRef    string
	ContentVersion int
	Producer       Producer
	// Subject is the business object the notice is about (zero if none). It is
	// not the recipient.
	Subject ObjectRef
	// RecipientKind, UserID and Recipient identify who the notice is for. UserID
	// is zero and Recipient is zero for a direct address.
	RecipientKind RecipientKind
	UserID        uint
	Recipient     ObjectRef
	Channel       Channel
	// Address is the concrete target: an email, a phone number or a push token.
	Address string
	// DeviceID identifies the push device behind Address; empty otherwise.
	DeviceID  string
	Language  string
	Level     notification.Level
	Title     string
	Body      string
	ActionURL string
	// Subject, HTML and Text are the email/SMS content; Data is the push payload.
	EmailSubject string
	HTML         string
	Text         string
	Data         map[string]string
}

// Receipt is what a provider returned for an accepted message.
type Receipt struct{ ProviderMessageID string }

// ChannelAdapter sends one message through a provider. Return an error wrapping
// ErrPermanent when retrying cannot succeed and ErrAmbiguous when the provider
// may have accepted the message. Adapters must be safe for concurrent use and
// must not be called inside a database transaction.
type ChannelAdapter interface {
	Channel() Channel
	Send(context.Context, Message) (Receipt, error)
}

// AddressResolver finds where a user is reachable on email or SMS: an email
// address or a phone number. Return ErrNoAddress when none. Push devices come
// from the device registry; a push address returned here is a legacy single
// token used only when the user has no registered device.
type AddressResolver interface {
	Resolve(ctx context.Context, tenantID uuid.UUID, userID uint, channel Channel) (string, error)
}

// Delivery is the persisted state of one notice, recipient, channel and target.
type Delivery struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	NoticeID uuid.UUID
	// EventKey, EventID and ContentVersion denormalize the notice identity for lookups.
	EventKey       string
	EventID        string
	ContentVersion int
	RecipientKind  RecipientKind
	// UserID is zero unless the recipient is a user.
	UserID uint
	// Record is set for record recipients.
	Record *ObjectRef
	// RecipientKey identifies the recipient within the notice.
	RecipientKey string
	Channel      Channel
	// TargetKey identifies the target within the recipient and channel; empty
	// until the recipient is expanded.
	TargetKey      string
	TargetAddress  string
	TargetDeviceID string
	Status         DeliveryStatus
	Language       string
	Level          notification.Level
	Title          string
	Body           string
	ActionURL      string
	Content        ChannelContent
	Attempts       int
	// AttemptLimit is the attempt count at which automatic retries stop.
	AttemptLimit    int
	ErrorCode       string
	ProviderReceipt string
	// InboxNotificationID links an in-app delivery to its inbox row.
	InboxNotificationID uuid.UUID
	CreatedAt           time.Time
	AvailableAt         time.Time
	DeliveredAt         *time.Time
}

// Attempt is one provider call for a delivery. An attempt without an outcome is
// in flight; one whose lease expired is recorded as ambiguous.
type Attempt struct {
	DeliveryID      uuid.UUID
	Number          int
	StartedAt       time.Time
	FinishedAt      *time.Time
	Outcome         AttemptOutcome
	ProviderReceipt string
	ErrorCode       string
}

// AttemptResult is what the worker records for a claimed attempt.
type AttemptResult struct {
	Outcome AttemptOutcome
	// Status is the delivery state after the attempt.
	Status    DeliveryStatus
	Receipt   string
	ErrorCode string
	// RetryIn schedules the next attempt when Status is pending.
	RetryIn time.Duration
}

// DeliveryStats counts a tenant's deliveries per channel and status.
type DeliveryStats struct {
	Channel Channel
	Status  DeliveryStatus
	Count   int64
}

// DeliveryRepository persists per-target delivery state. Create runs inside the
// caller's transaction; claims and records never hold a transaction across a
// resolver, renderer or provider call.
type DeliveryRepository interface {
	// Create stores deliveries idempotently by notice, recipient, channel and target.
	Create(context.Context, []Delivery) error
	// ClaimUnresolved leases up to limit due unresolved recipient/channel rows
	// and counts the attempt. It first fails rows that exhausted their attempts.
	ClaimUnresolved(ctx context.Context, limit int, lease time.Duration) ([]Delivery, error)
	// Expand replaces a claimed unresolved row by one delivery per target. A
	// claim that was superseded is dropped.
	Expand(ctx context.Context, claimed Delivery, targets []Delivery) error
	// Settle ends or reschedules a claimed unresolved row without targets: no
	// address, skipped, failed, or unresolved again after retryIn.
	Settle(ctx context.Context, claimed Delivery, status DeliveryStatus, errorCode string, retryIn time.Duration) error
	// Claim leases up to limit due pending external deliveries, counts the
	// attempt and opens its attempt record. It first fails deliveries that
	// exhausted their attempts instead of leasing them. A previous attempt that
	// never recorded an outcome becomes ambiguous.
	Claim(ctx context.Context, limit int, lease time.Duration) ([]Delivery, error)
	// Record stores the outcome of the claimed attempt number; an outcome whose
	// claim was superseded (lease expired and re-claimed, or retried) is dropped.
	Record(ctx context.Context, id uuid.UUID, attempt int, result AttemptResult) error
	// Retry reopens one failed, ambiguous or unreachable delivery with a fresh
	// attempt budget, leaving siblings alone.
	Retry(context.Context, uuid.UUID) error
	// ForEvent lists the deliveries of one event.
	ForEvent(ctx context.Context, eventKey, eventID string) ([]Delivery, error)
	// Attempts lists the provider attempts of one delivery.
	Attempts(ctx context.Context, deliveryID uuid.UUID) ([]Attempt, error)
	Stats(context.Context) ([]DeliveryStats, error)
}

// NoticeMode says how a notice chose its recipients.
type NoticeMode string

const (
	// NoticeRule publishes an event to the audience of the administrator's rule.
	NoticeRule NoticeMode = "rule"
	// NoticeExplicit sends to recipients named by the producer; rules never
	// retarget it.
	NoticeExplicit NoticeMode = "explicit"
)

// Notice is the persisted identity of one notice instance: which notice and
// version, which snapshot, who produced it and what it is about.
type Notice struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	EventKey    string
	Version     int
	SnapshotRef string
	// EventID is the dedupe identity of the notice within the event key.
	EventID   string
	Mode      NoticeMode
	Subject   ObjectRef
	Producer  Producer
	Event     Event
	CreatedAt time.Time
}

// NoticeRepository persists notice instances. Ensure runs in the caller's
// transaction, so rollback removes the notice together with its intent.
type NoticeRepository interface {
	// Ensure stores the notice once per tenant, event key and event ID and
	// returns the stored row, which may be an earlier identical enqueue.
	Ensure(context.Context, Notice) (Notice, error)
	Get(ctx context.Context, id uuid.UUID) (Notice, error)
}

// PushPlatform is the platform of a registered push device.
type PushPlatform string

const (
	PushIOS     PushPlatform = "ios"
	PushAndroid PushPlatform = "android"
	PushWeb     PushPlatform = "web"
)

// Valid reports whether p is a known platform.
func (p PushPlatform) Valid() bool { return p == PushIOS || p == PushAndroid || p == PushWeb }

// PushDevice is one push registration of a user. One user can have several.
type PushDevice struct {
	TenantID  uuid.UUID
	UserID    uint
	DeviceID  string
	Token     string
	Platform  PushPlatform
	CreatedAt time.Time
	RevokedAt *time.Time
}

// PushDeviceRepository is the device registry for user recipients. External
// recipients get their devices from a ContactResolver instead; they are never
// modelled as fake users.
type PushDeviceRepository interface {
	// Register stores or refreshes a device. A token held by another active
	// registration of the tenant moves to this device.
	Register(context.Context, PushDevice) error
	// Revoke stops pushes to a device of the user.
	Revoke(ctx context.Context, userID uint, deviceID string) error
	// Active lists the user's devices that are not revoked.
	Active(ctx context.Context, userID uint) ([]PushDevice, error)
}

// PreferenceRepository stores per-user channel opt-outs. A user without rows
// receives every channel selected by the rule or addressed request.
type PreferenceRepository interface {
	// Disabled returns the channels each of the given users opted out of.
	Disabled(ctx context.Context, userIDs []uint) (map[uint][]Channel, error)
	// SetDisabled replaces a user's opt-outs.
	SetDisabled(ctx context.Context, userID uint, channels []Channel) error
}
