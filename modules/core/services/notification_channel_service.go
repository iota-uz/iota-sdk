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

// NotificationChannelService owns external-channel delivery: it persists one
// delivery per event/recipient/channel inside the producer's transaction and
// calls provider adapters only after that transaction committed.
type NotificationChannelService struct {
	repo     notifications.DeliveryRepository
	adapters map[notifications.Channel]notifications.ChannelAdapter
	resolver notifications.AddressResolver
	logger   *logrus.Logger
}

func NewNotificationChannelService(repo notifications.DeliveryRepository, adapters []notifications.ChannelAdapter, resolver notifications.AddressResolver, logger *logrus.Logger) *NotificationChannelService {
	index := make(map[notifications.Channel]notifications.ChannelAdapter, len(adapters))
	for _, a := range adapters {
		if a != nil && a.Channel().External() {
			index[a.Channel()] = a
		}
	}
	return &NotificationChannelService{repo: repo, adapters: index, resolver: resolver, logger: logger}
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

// Process attempts one batch of due external deliveries for the tenant in ctx.
// The claim commits before any provider call, so no transaction is open while
// providers run. Delivery is at-least-once: a crash between a provider
// accepting a message and the outcome being stored repeats that one message.
func (s *NotificationChannelService) Process(ctx context.Context) (int, error) {
	const op serrors.Op = "NotificationChannelService.Process"
	if ctx.Value(constants.TxKey) != nil {
		return 0, serrors.New(serrors.FailedPrecondition, "Channel processing requires its own transaction").WithOp(op)
	}
	var claimed []notifications.Delivery
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		claimed, err = s.repo.Claim(txCtx, notificationChannelBatchSize, notificationChannelLease)
		return err
	}); err != nil {
		return 0, serrors.Wrap(op, err)
	}
	var errs []error
	for _, d := range claimed {
		if ctx.Err() != nil {
			break
		}
		if err := s.attempt(ctx, d); err != nil {
			errs = append(errs, err)
		}
	}
	return len(claimed), errors.Join(errs...)
}

func (s *NotificationChannelService) attempt(ctx context.Context, d notifications.Delivery) error {
	const op serrors.Op = "NotificationChannelService.attempt"
	out := s.send(ctx, d)
	record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.repo.Record(record, d.ID, out.status, out.receipt, out.lastError, out.retryIn); err != nil {
		return serrors.WrapContext(op, err, "failed to record notification delivery outcome")
	}
	return nil
}

type deliveryOutcome struct {
	status    notifications.DeliveryStatus
	receipt   string
	lastError string
	retryIn   time.Duration
}

func (s *NotificationChannelService) send(ctx context.Context, d notifications.Delivery) deliveryOutcome {
	retry := func(reason string) deliveryOutcome {
		if d.Attempts >= notifications.MaxDeliveryAttempts {
			return deliveryOutcome{status: notifications.DeliveryFailed, lastError: reason}
		}
		return deliveryOutcome{status: notifications.DeliveryPending, lastError: reason + "; retry scheduled", retryIn: time.Duration(min(300, 1<<min(d.Attempts, 8))) * time.Second}
	}
	noAddress := deliveryOutcome{status: notifications.DeliveryNoAddress, lastError: "recipient has no address for channel"}
	adapter, ok := s.adapters[d.Channel]
	if !ok {
		return retry("channel adapter is not installed")
	}
	if s.resolver == nil {
		return noAddress
	}
	sendCtx, cancel := context.WithTimeout(ctx, notificationChannelSendLimit)
	defer cancel()
	address, err := s.resolver.Resolve(sendCtx, d.TenantID, d.UserID, d.Channel)
	if errors.Is(err, notifications.ErrNoAddress) || (err == nil && address == "") {
		return noAddress
	}
	if err != nil {
		return retry("address lookup failed")
	}
	result, err := adapter.Send(sendCtx, notifications.Message{DeliveryID: d.ID, TenantID: d.TenantID, EventKey: d.EventKey, EventID: d.EventID, ContentVersion: d.ContentVersion, UserID: d.UserID, Channel: d.Channel, Address: address, Language: d.Language, Level: d.Level, Title: d.Title, Body: d.Body, ActionURL: d.ActionURL})
	switch {
	case err == nil:
		return deliveryOutcome{status: notifications.DeliverySent, receipt: result.ProviderMessageID}
	case errors.Is(err, notifications.ErrPermanent):
		return deliveryOutcome{status: notifications.DeliveryFailed, lastError: "provider rejected the message"}
	default:
		return retry("provider failure")
	}
}
