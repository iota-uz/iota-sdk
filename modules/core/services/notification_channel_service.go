package services

import (
	"context"
	"errors"
	"slices"
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
	adapters map[notifications.Channel]notifications.ChannelAdapter
	resolver notifications.AddressResolver
	platform *NotificationPlatform
	logger   *logrus.Logger
}

// WithPlatform installs the notice store, renderers, contact resolvers, device
// registry and policies the worker needs to turn recipients into targets.
func (s *NotificationChannelService) WithPlatform(platform *NotificationPlatform) *NotificationChannelService {
	s.platform = platform
	return s
}

func NewNotificationChannelService(repo notifications.DeliveryRepository, prefs notifications.PreferenceRepository, adapters []notifications.ChannelAdapter, resolver notifications.AddressResolver, logger *logrus.Logger) *NotificationChannelService {
	index := make(map[notifications.Channel]notifications.ChannelAdapter, len(adapters))
	for _, a := range adapters {
		if a != nil && a.Channel().External() {
			index[a.Channel()] = a
		}
	}
	return &NotificationChannelService{repo: repo, prefs: prefs, adapters: index, resolver: resolver, logger: logger}
}

// Available lists the channels a rule can select: in-app and every installed adapter.
func (s *NotificationChannelService) Available() []notifications.Channel {
	result := []notifications.Channel{notifications.ChannelInApp}
	for _, c := range notifications.Channels() {
		if _, ok := s.adapters[c]; ok {
			result = append(result, c)
		}
	}
	return result
}

// Validate rejects unknown, duplicated or not installed channels.
func (s *NotificationChannelService) Validate(channels []notifications.Channel) error {
	const op serrors.Op = "NotificationChannelService.Validate"
	available := s.Available()
	seen := map[notifications.Channel]bool{}
	for _, c := range channels {
		if !c.Valid() || seen[c] || !slices.Contains(available, c) {
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

// SetEnabled stores which of the available channels a user wants. Opt-outs for
// channels without an installed adapter are kept, so installing one later does
// not override an earlier choice.
func (s *NotificationChannelService) SetEnabled(ctx context.Context, userID uint, enabled []notifications.Channel) error {
	const op serrors.Op = "NotificationChannelService.SetEnabled"
	if err := s.Validate(enabled); err != nil {
		return serrors.Wrap(op, err)
	}
	current, err := s.prefs.Disabled(ctx, []uint{userID})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	available := s.Available()
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

func (s *NotificationChannelService) send(ctx context.Context, d notifications.Delivery, notices map[uuid.UUID]notifications.Notice) notifications.AttemptResult {
	adapter, ok := s.adapters[d.Channel]
	if !ok {
		return s.failedAttempt(d, notifications.ErrorCodeAdapterMissing)
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
	switch {
	case err == nil:
		return notifications.AttemptResult{Outcome: notifications.AttemptSent, Status: notifications.DeliverySent, Receipt: receipt.ProviderMessageID}
	case errors.Is(err, notifications.ErrPermanent):
		return notifications.AttemptResult{Outcome: notifications.AttemptPermanent, Status: notifications.DeliveryFailed, ErrorCode: notifications.ErrorCodeProviderRejected}
	case errors.Is(err, notifications.ErrAmbiguous) || errors.Is(err, context.DeadlineExceeded):
		// The provider may have accepted the message. Retrying automatically
		// could send it twice, so an operator decides.
		return notifications.AttemptResult{Outcome: notifications.AttemptAmbiguous, Status: notifications.DeliveryAmbiguous, ErrorCode: notifications.ErrorCodeProviderAmbiguous}
	default:
		return s.failedAttempt(d, notifications.ErrorCodeProviderFailure)
	}
}
