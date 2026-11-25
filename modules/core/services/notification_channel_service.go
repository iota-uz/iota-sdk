package services

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

const (
	notificationChannelBatchSize = 50
	notificationChannelLease     = 2 * time.Minute
	notificationChannelSendLimit = 30 * time.Second
)

// NotificationChannelService owns external-channel delivery. It persists a
// delivery intent per notice, recipient and channel inside the producer's
// transaction. After commit its worker expands each intent into targets (the
// user's addresses and devices, a record's contacts, a direct address), applies
// policy, renders the content with the pinned definition and calls provider
// adapters, recording every attempt. No step holds a database transaction while
// it calls a resolver, renderer or provider.
type NotificationChannelService struct {
	repo     notifications.DeliveryRepository
	prefs    notifications.PreferenceRepository
	registry notifications.ChannelRegistry
	links    notifications.ChannelLinkRepository
	conns    notifications.ChannelConnections
	resolver notifications.AddressResolver
	platform *NotificationPlatform
	logger   *logrus.Logger
}

// WithRegistry replaces the channel registry built from the constructor's
// adapters: the application's registry, with contributed channels.
func (s *NotificationChannelService) WithRegistry(registry notifications.ChannelRegistry) *NotificationChannelService {
	if registry != nil {
		s.registry = registry
	}
	return s
}

// WithLinks installs the store of linked channel addresses (telegram).
func (s *NotificationChannelService) WithLinks(links notifications.ChannelLinkRepository) *NotificationChannelService {
	s.links = links
	return s
}

// WithConnections installs how connector-backed channels find the tenant's
// connection at sending time.
func (s *NotificationChannelService) WithConnections(conns notifications.ChannelConnections) *NotificationChannelService {
	s.conns = conns
	return s
}

// Registry is the channel registry the service sends through.
func (s *NotificationChannelService) Registry() notifications.ChannelRegistry { return s.registry }

// WithPlatform installs the notice store, renderers, contact resolvers, device
// registry and policies the worker needs to turn recipients into targets.
func (s *NotificationChannelService) WithPlatform(platform *NotificationPlatform) *NotificationChannelService {
	s.platform = platform
	return s
}

func NewNotificationChannelService(repo notifications.DeliveryRepository, prefs notifications.PreferenceRepository, adapters []notifications.ChannelAdapter, resolver notifications.AddressResolver, logger *logrus.Logger) *NotificationChannelService {
	var external []notifications.ChannelAdapter
	for _, a := range adapters {
		if a != nil && a.Channel().External() {
			external = append(external, a)
		}
	}
	registry, err := notifications.NewChannelRegistry(external...)
	if err != nil {
		// Two adapters for one channel: the first wins, as before the registry.
		registry, _ = notifications.NewChannelRegistry()
		seen := map[notifications.Channel]bool{}
		for _, a := range external {
			if !seen[a.Channel()] {
				seen[a.Channel()] = true
				_ = registry.Register(notifications.ChannelSpec{Channel: a.Channel()}, a)
			}
		}
	}
	return &NotificationChannelService{repo: repo, prefs: prefs, registry: registry, resolver: resolver, logger: logger}
}

// Available lists the channels the tenant in ctx can select: in-app, every
// installed adapter and the connector-backed channels the tenant has a
// connection for.
func (s *NotificationChannelService) Available(ctx context.Context) ([]notifications.Channel, error) {
	specs, err := s.AvailableSpecs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]notifications.Channel, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Channel)
	}
	return out, nil
}

// AvailableSpecs is Available with the channels' specs.
func (s *NotificationChannelService) AvailableSpecs(ctx context.Context) ([]notifications.ChannelSpec, error) {
	const op serrors.Op = "NotificationChannelService.AvailableSpecs"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	specs, err := s.registry.ForTenant(ctx, tenant)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return specs, nil
}

// Validate rejects malformed, unregistered, duplicated or unavailable channels.
func (s *NotificationChannelService) Validate(ctx context.Context, channels []notifications.Channel) error {
	const op serrors.Op = "NotificationChannelService.Validate"
	for _, c := range channels {
		if _, _, ok := s.registry.Lookup(c); !c.Valid() || !ok {
			return serrors.New(serrors.Invalid, "invalid or unavailable notification channel").WithOp(op)
		}
	}
	available, err := s.Available(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	seen := map[notifications.Channel]bool{}
	for _, c := range channels {
		if seen[c] || !slices.Contains(available, c) {
			return serrors.New(serrors.Invalid, "invalid or unavailable notification channel").WithOp(op)
		}
		seen[c] = true
	}
	return nil
}

// Disabled returns each user's opted-out channels.
func (s *NotificationChannelService) Disabled(ctx context.Context, userIDs []uint) (map[uint][]notifications.Channel, error) {
	return s.prefs.Disabled(ctx, userIDs)
}

// SetEnabled stores which of the available channels a user wants. Choices for
// channels the tenant cannot use now are kept, so installing or connecting one
// later does not override an earlier choice.
func (s *NotificationChannelService) SetEnabled(ctx context.Context, userID uint, enabled []notifications.Channel) error {
	const op serrors.Op = "NotificationChannelService.SetEnabled"
	if err := s.Validate(ctx, enabled); err != nil {
		return serrors.Wrap(op, err)
	}
	current, err := s.prefs.Disabled(ctx, []uint{userID})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	available, err := s.Available(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	var disabled []notifications.Channel
	for _, c := range current[userID] {
		if !slices.Contains(available, c) {
			disabled = append(disabled, c)
		}
	}
	for _, c := range available {
		if !slices.Contains(enabled, c) {
			disabled = append(disabled, c)
		}
	}
	return s.prefs.SetDisabled(ctx, userID, disabled)
}

// SetChannelEnabled turns one available channel on or off for a user, keeping
// the other choices.
func (s *NotificationChannelService) SetChannelEnabled(ctx context.Context, userID uint, channel notifications.Channel, enabled bool) error {
	const op serrors.Op = "NotificationChannelService.SetChannelEnabled"
	if err := s.Validate(ctx, []notifications.Channel{channel}); err != nil {
		return serrors.Wrap(op, err)
	}
	// Read and write under the user's lock: two switches flipped at once both land.
	return serrors.Wrap(op, composables.InTx(ctx, func(txCtx context.Context) error {
		if err := lockPreferences(txCtx, userID); err != nil {
			return err
		}
		current, err := s.prefs.Disabled(txCtx, []uint{userID})
		if err != nil {
			return err
		}
		disabled := slices.DeleteFunc(slices.Clone(current[userID]), func(c notifications.Channel) bool { return c == channel })
		if !enabled {
			disabled = append(disabled, channel)
		}
		return s.prefs.SetDisabled(txCtx, userID, disabled)
	}))
}

// lockPreferences serializes the channel choices of one user of the tenant in ctx until the transaction ends.
func lockPreferences(ctx context.Context, userID uint) error {
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('core.notification_channel_preferences:' || $1::text || ':' || $2::text, 0))`, tenant.String(), strconv.FormatUint(uint64(userID), 10))
	return err
}

// Record persists deliveries in the caller's transaction.
func (s *NotificationChannelService) Record(ctx context.Context, deliveries []notifications.Delivery) error {
	return s.repo.Create(ctx, deliveries)
}

// Retry reopens one delivery without touching other channels of the same event.
func (s *NotificationChannelService) Retry(ctx context.Context, id uuid.UUID) error {
	return s.repo.Retry(ctx, id)
}

func (s *NotificationChannelService) Deliveries(ctx context.Context, eventKey, eventID string) ([]notifications.Delivery, error) {
	return s.repo.ForEvent(ctx, eventKey, eventID)
}

// NoticeDeliveries lists the deliveries of one notice.
func (s *NotificationChannelService) NoticeDeliveries(ctx context.Context, noticeID uuid.UUID) ([]notifications.Delivery, error) {
	notice, err := s.notice(ctx, noticeID, map[uuid.UUID]notifications.Notice{})
	if err != nil {
		return nil, err
	}
	return s.repo.ForEvent(ctx, notice.EventKey, notice.EventID)
}

func (s *NotificationChannelService) Stats(ctx context.Context) ([]notifications.DeliveryStats, error) {
	return s.repo.Stats(ctx)
}

// RegisterDevice stores or refreshes a push device of a user.
func (s *NotificationChannelService) RegisterDevice(ctx context.Context, device notifications.PushDevice) error {
	const op serrors.Op = "NotificationChannelService.RegisterDevice"
	if s.platform == nil || s.platform.Devices == nil {
		return serrors.New(serrors.FailedPrecondition, "push device registry is not configured").WithOp(op)
	}
	if device.UserID == 0 || !device.Platform.Valid() || device.DeviceID == "" || device.Token == "" || len(device.DeviceID) > 200 || len(device.Token) > 4096 {
		return serrors.New(serrors.Invalid, "invalid push device").WithOp(op)
	}
	return s.platform.Devices.Register(ctx, device)
}

// RevokeDevice stops pushes to one device of a user.
func (s *NotificationChannelService) RevokeDevice(ctx context.Context, userID uint, deviceID string) error {
	const op serrors.Op = "NotificationChannelService.RevokeDevice"
	if s.platform == nil || s.platform.Devices == nil {
		return serrors.New(serrors.FailedPrecondition, "push device registry is not configured").WithOp(op)
	}
	return s.platform.Devices.Revoke(ctx, userID, deviceID)
}

// Attempts lists the provider attempts of one delivery.
func (s *NotificationChannelService) Attempts(ctx context.Context, deliveryID uuid.UUID) ([]notifications.Attempt, error) {
	return s.repo.Attempts(ctx, deliveryID)
}

// Process runs one batch of the external-channel worker for the tenant in ctx:
// it resolves due recipients into targets, then attempts due target deliveries.
// Every claim commits before any resolver, renderer or provider call, so no
// transaction is open while they run. Delivery is at-least-once: a crash between
// a provider accepting a message and the outcome being stored repeats that one
// message after the lease expires, and the earlier attempt is recorded as
// ambiguous. It returns the number of provider attempts made.
func (s *NotificationChannelService) Process(ctx context.Context) (int, error) {
	const op serrors.Op = "NotificationChannelService.Process"
	if ctx.Value(constants.TxKey) != nil {
		return 0, serrors.New(serrors.FailedPrecondition, "Channel processing requires its own transaction").WithOp(op)
	}
	notices := map[uuid.UUID]notifications.Notice{}
	var errs []error
	if err := s.resolve(ctx, notices); err != nil {
		errs = append(errs, err)
	}
	var claimed []notifications.Delivery
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		claimed, err = s.repo.Claim(txCtx, notificationChannelBatchSize, notificationChannelLease)
		return err
	}); err != nil {
		return 0, serrors.Wrap(op, errors.Join(append(errs, err)...))
	}
	for _, d := range claimed {
		if ctx.Err() != nil {
			break
		}
		if err := s.attempt(ctx, d, notices); err != nil {
			errs = append(errs, err)
		}
	}
	return len(claimed), errors.Join(errs...)
}

func (s *NotificationChannelService) notice(ctx context.Context, id uuid.UUID, cache map[uuid.UUID]notifications.Notice) (notifications.Notice, error) {
	if n, ok := cache[id]; ok {
		return n, nil
	}
	if s.platform == nil || s.platform.Notices == nil {
		return notifications.Notice{}, serrors.New(serrors.FailedPrecondition, "notification platform is not configured")
	}
	n, err := s.platform.Notices.Get(ctx, id)
	if err != nil {
		return n, err
	}
	cache[id] = n
	return n, nil
}

func retryBackoff(attempts int) time.Duration {
	return time.Duration(min(300, 1<<min(max(attempts, 0), 8))) * time.Second
}

func (s *NotificationChannelService) attempt(ctx context.Context, d notifications.Delivery, notices map[uuid.UUID]notifications.Notice) error {
	const op serrors.Op = "NotificationChannelService.attempt"
	result := s.send(ctx, d, notices)
	record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.repo.Record(record, d.ID, d.Attempts, result); err != nil {
		return serrors.WrapContext(op, err, "failed to record notification delivery outcome")
	}
	return nil
}

func (s *NotificationChannelService) failedAttempt(d notifications.Delivery, code string) notifications.AttemptResult {
	if d.Attempts >= d.AttemptLimit {
		return notifications.AttemptResult{Outcome: notifications.AttemptFailed, Status: notifications.DeliveryFailed, ErrorCode: code}
	}
	return notifications.AttemptResult{Outcome: notifications.AttemptFailed, Status: notifications.DeliveryPending, ErrorCode: code, RetryIn: retryBackoff(d.Attempts)}
}

// notConfigured is the outcome of a delivery on a channel the tenant has no
// connection for any more.
var notConfigured = notifications.AttemptResult{Outcome: notifications.AttemptPermanent, Status: notifications.DeliverySkipped, ErrorCode: notifications.ErrorCodeChannelNotConfigured}

// connected reports whether a connector-backed channel has a connection for
// the delivery's tenant now; other channels always do.
func (s *NotificationChannelService) connected(ctx context.Context, spec notifications.ChannelSpec, tenantID uuid.UUID) (bool, error) {
	if spec.ConnectorKind == "" {
		return true, nil
	}
	if s.conns == nil {
		return false, nil
	}
	_, ok, err := s.conns.Connection(ctx, tenantID, spec)
	return ok, err
}

// stillLinked reports whether the address of a queued delivery on a linked channel is still the user's link
// through the connection the channel sends through now: a delivery queued before the user unlinked, or before
// the tenant changed the bot, is not sent through a bot the user never linked with.
func (s *NotificationChannelService) stillLinked(ctx context.Context, spec notifications.ChannelSpec, d notifications.Delivery) (bool, error) {
	if spec.Address != notifications.AddressLinked || d.RecipientKind != notifications.RecipientUser {
		return true, nil
	}
	if s.links == nil {
		return false, nil
	}
	link, err := s.links.Get(ctx, d.UserID, d.Channel)
	if errors.Is(err, notifications.ErrNoAddress) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if link.Address != d.TargetAddress {
		return false, nil
	}
	if spec.ConnectorKind == "" || s.conns == nil {
		return true, nil
	}
	current, ok, err := s.conns.Connection(ctx, d.TenantID, spec)
	if err != nil {
		return false, err
	}
	return ok && current == link.ConnectionID, nil
}

func (s *NotificationChannelService) send(ctx context.Context, d notifications.Delivery, notices map[uuid.UUID]notifications.Notice) notifications.AttemptResult {
	spec, adapter, ok := s.registry.Lookup(d.Channel)
	if !ok || adapter == nil {
		return s.failedAttempt(d, notifications.ErrorCodeAdapterMissing)
	}
	switch connected, err := s.connected(ctx, spec, d.TenantID); {
	case err != nil:
		return s.failedAttempt(d, notifications.ErrorCodeProviderFailure)
	case !connected:
		return notConfigured
	}
	switch linked, err := s.stillLinked(ctx, spec, d); {
	case err != nil:
		return s.failedAttempt(d, notifications.ErrorCodeProviderFailure)
	case !linked:
		return notifications.AttemptResult{Outcome: notifications.AttemptPermanent, Status: notifications.DeliveryNoAddress, ErrorCode: notifications.ErrorCodeNoAddress}
	}
	notice, err := s.notice(ctx, d.NoticeID, notices)
	if err != nil {
		return s.failedAttempt(d, notifications.ErrorCodeProviderFailure)
	}
	sendCtx, cancel := context.WithTimeout(ctx, notificationChannelSendLimit)
	defer cancel()
	message := notifications.Message{
		DeliveryID: d.ID, IdempotencyKey: d.ID.String(), Attempt: d.Attempts, TenantID: d.TenantID, NoticeID: d.NoticeID,
		EventKey: notice.EventKey, EventID: notice.EventID, SnapshotRef: notice.SnapshotRef, ContentVersion: d.ContentVersion,
		Producer: notice.Producer, Subject: notice.Subject, RecipientKind: d.RecipientKind, UserID: d.UserID,
		Channel: d.Channel, Address: d.TargetAddress, DeviceID: d.TargetDeviceID, Language: d.Language, Level: d.Level,
		Title: d.Title, Body: d.Body, ActionURL: d.ActionURL,
		EmailSubject: d.Content.Subject, HTML: d.Content.HTML, Text: d.Content.Text, Data: d.Content.Data,
	}
	if d.Record != nil {
		message.Recipient = *d.Record
	}
	receipt, err := adapter.Send(sendCtx, message)
	var later *notifications.RetryAfterError
	switch {
	case err == nil:
		return notifications.AttemptResult{Outcome: notifications.AttemptSent, Status: notifications.DeliverySent, Receipt: receipt.ProviderMessageID}
	case errors.Is(err, notifications.ErrChannelUnavailable):
		return notConfigured
	case errors.As(err, &later):
		result := s.failedAttempt(d, notifications.ErrorCodeProviderFailure)
		if result.Status == notifications.DeliveryPending && later.After > 0 {
			result.RetryIn = later.After
		}
		return result
	case errors.Is(err, notifications.ErrPermanent):
		if errors.Is(err, notifications.ErrTargetGone) {
			if spec.Address == notifications.AddressLinked {
				s.revokeGoneLink(ctx, spec, d)
			} else {
				s.revokeGoneDevice(ctx, d)
			}
		}
		return notifications.AttemptResult{Outcome: notifications.AttemptPermanent, Status: notifications.DeliveryFailed, ErrorCode: notifications.ErrorCodeProviderRejected}
	case errors.Is(err, notifications.ErrAmbiguous) || errors.Is(err, context.DeadlineExceeded):
		// The provider may have accepted the message. Retrying automatically
		// could send it twice, so an operator decides.
		return notifications.AttemptResult{Outcome: notifications.AttemptAmbiguous, Status: notifications.DeliveryAmbiguous, ErrorCode: notifications.ErrorCodeProviderAmbiguous}
	default:
		return s.failedAttempt(d, notifications.ErrorCodeProviderFailure)
	}
}

// revokeGoneDevice stops pushes to a registered device its provider reported as
// gone. Failing to revoke only means the next notice learns the same thing.
func (s *NotificationChannelService) revokeGoneDevice(ctx context.Context, d notifications.Delivery) {
	if d.RecipientKind != notifications.RecipientUser || d.TargetDeviceID == "" || s.platform == nil || s.platform.Devices == nil {
		return
	}
	revoke, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.platform.Devices.Revoke(revoke, d.UserID, d.TargetDeviceID); err != nil && s.logger != nil {
		s.logger.WithError(err).Warn("failed to revoke a push device its provider reported as gone")
	}
}

// revokeGoneLink ends the link of an address its provider reported gone (the
// user blocked the bot): later notices find no address, and the user sees why
// in the profile.
func (s *NotificationChannelService) revokeGoneLink(ctx context.Context, spec notifications.ChannelSpec, d notifications.Delivery) {
	if s.links == nil || d.TargetAddress == "" {
		return
	}
	revoke, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	connection := ""
	if s.conns != nil {
		if id, ok, err := s.conns.Connection(revoke, d.TenantID, spec); err == nil && ok {
			connection = id
		}
	}
	if err := s.links.DeleteByAddress(revoke, d.Channel, connection, d.TargetAddress); err != nil && s.logger != nil {
		s.logger.WithError(err).Warn("failed to remove a channel link its provider reported as gone")
	}
}
