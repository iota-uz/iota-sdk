package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// resolveProblem ends or reschedules the resolution of one recipient/channel.
type resolveProblem struct {
	status    notifications.DeliveryStatus
	code      string
	transient bool
}

var policyReasonPattern = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// resolve expands due recipient/channel intents into target deliveries. The
// claim commits first; resolvers, policies and renderers then run with no
// transaction open, and each result is stored in its own short transaction.
func (s *NotificationChannelService) resolve(ctx context.Context, notices map[uuid.UUID]notifications.Notice) error {
	const op serrors.Op = "NotificationChannelService.resolve"
	var claimed []notifications.Delivery
	if err := composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		claimed, err = s.repo.ClaimUnresolved(txCtx, notificationChannelBatchSize, notificationChannelLease)
		return err
	}); err != nil {
		return serrors.Wrap(op, err)
	}
	var errs []error
	for _, d := range claimed {
		if ctx.Err() != nil {
			break
		}
		if err := s.resolveOne(ctx, d, notices); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *NotificationChannelService) resolveOne(ctx context.Context, d notifications.Delivery, notices map[uuid.UUID]notifications.Notice) error {
	const op serrors.Op = "NotificationChannelService.resolveOne"
	store, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	settle := func(p resolveProblem) error {
		if p.transient {
			if d.Attempts >= d.AttemptLimit {
				return serrors.Wrap(op, s.repo.Settle(store, d, notifications.DeliveryFailed, notifications.ErrorCodeResolutionLimit, 0))
			}
			return serrors.Wrap(op, s.repo.Settle(store, d, notifications.DeliveryUnresolved, p.code, retryBackoff(d.Attempts)))
		}
		return serrors.Wrap(op, s.repo.Settle(store, d, p.status, p.code, 0))
	}
	notice, err := s.notice(ctx, d.NoticeID, notices)
	if err != nil {
		return settle(resolveProblem{code: notifications.ErrorCodeResolutionFailed, transient: true})
	}
	targets, language, problem := s.targets(ctx, d)
	if problem != nil {
		return settle(*problem)
	}
	if len(targets) == 0 {
		return settle(resolveProblem{status: notifications.DeliveryNoAddress, code: notifications.ErrorCodeNoAddress})
	}
	rows := make([]notifications.Delivery, 0, len(targets))
	allowed := make([]notifications.Delivery, 0, len(targets))
	for _, target := range targets {
		row := d
		row.ID, row.Attempts, row.ErrorCode = uuid.Nil, 0, ""
		row.TargetKey, row.TargetAddress, row.TargetDeviceID = target.Key(), target.Address, target.DeviceID
		row.Language = language
		row.Status = notifications.DeliveryPending
		if d.RecipientKind != notifications.RecipientUser {
			decision, err := s.platform.policy().Evaluate(ctx, notifications.PolicyRequest{TenantID: d.TenantID, EventKey: notice.EventKey, SnapshotRef: notice.SnapshotRef, Subject: notice.Subject, Recipient: recipientOf(d), Target: target})
			if err != nil {
				return settle(resolveProblem{code: notifications.ErrorCodeResolutionFailed, transient: true})
			}
			if !decision.Allow {
				row.Status = notifications.DeliverySkipped
				row.ErrorCode = notifications.ErrorCodePolicyDenied
				if policyReasonPattern.MatchString(decision.Reason) {
					row.ErrorCode = notifications.ErrorCodePolicyDenied + ":" + decision.Reason
				}
			}
		}
		rows = append(rows, row)
		if row.Status == notifications.DeliveryPending {
			allowed = append(allowed, row)
		}
	}
	if len(allowed) > 0 {
		content, err := s.platform.Renderers.Render(ctx, notifications.RenderRequest{Event: notice.Event, Channel: d.Channel, Language: language})
		if err != nil {
			if errors.Is(err, notifications.ErrPermanent) || errors.Is(err, notifications.ErrUnknownDefinition) {
				return settle(resolveProblem{status: notifications.DeliveryFailed, code: notifications.ErrorCodeResolutionRender})
			}
			return settle(resolveProblem{code: notifications.ErrorCodeResolutionFailed, transient: true})
		}
		// Every channel, not only in-app, must carry a safe internal link.
		if _, err := notification.New(1, "validation", "", notification.WithActionURL(content.ActionURL)); err != nil {
			return settle(resolveProblem{status: notifications.DeliveryFailed, code: notifications.ErrorCodeResolutionBadContent})
		}
		title, body := content.Summary(d.Channel)
		for i := range rows {
			if rows[i].Status != notifications.DeliveryPending {
				continue
			}
			rows[i].Content, rows[i].Title, rows[i].Body, rows[i].ActionURL = content, truncateRunes(title, 300), body, content.ActionURL
		}
	}
	return serrors.Wrap(op, s.repo.Expand(store, d, rows))
}

func recipientOf(d notifications.Delivery) notifications.Addressee {
	switch d.RecipientKind {
	case notifications.RecipientUser:
		return notifications.UserAddressee(d.UserID)
	case notifications.RecipientRecord:
		if d.Record != nil {
			return notifications.RecordAddressee(d.Record.Entity, d.Record.Key)
		}
	}
	return notifications.AddressAddressee(map[notifications.Channel]string{d.Channel: d.TargetAddress})
}

// targets expands a recipient on one channel into concrete targets.
func (s *NotificationChannelService) targets(ctx context.Context, d notifications.Delivery) ([]notifications.Target, string, *resolveProblem) {
	transient := &resolveProblem{code: notifications.ErrorCodeResolutionFailed, transient: true}
	language := d.Language
	switch d.RecipientKind {
	case notifications.RecipientAddress:
		return []notifications.Target{{Channel: d.Channel, Address: d.TargetAddress}}, language, nil
	case notifications.RecipientRecord:
		if d.Record == nil || s.platform == nil {
			return nil, language, &resolveProblem{status: notifications.DeliveryFailed, code: notifications.ErrorCodeResolverMissing}
		}
		resolver, ok := s.platform.Contacts.Lookup(d.Record.Entity)
		if !ok {
			return nil, language, &resolveProblem{status: notifications.DeliveryFailed, code: notifications.ErrorCodeResolverMissing}
		}
		contact, err := resolver.ResolveContact(ctx, d.TenantID, *d.Record)
		if errors.Is(err, notifications.ErrContactNotFound) {
			return nil, language, &resolveProblem{status: notifications.DeliveryNoAddress, code: notifications.ErrorCodeContactNotFound}
		}
		if err != nil {
			return nil, language, transient
		}
		if contact.Language != "" {
			language = contact.Language
		}
		return uniqueTargets(d.Channel, contact.Targets), language, nil
	}
	var found []notifications.Target
	if d.Channel == notifications.ChannelPush && s.platform != nil && s.platform.Devices != nil {
		devices, err := s.platform.Devices.Active(ctx, d.UserID)
		if err != nil {
			return nil, language, transient
		}
		for _, device := range devices {
			found = append(found, notifications.Target{Channel: notifications.ChannelPush, Address: device.Token, DeviceID: device.DeviceID})
		}
	}
	if len(found) == 0 && s.resolver != nil {
		address, err := s.resolver.Resolve(ctx, d.TenantID, d.UserID, d.Channel)
		switch {
		case errors.Is(err, notifications.ErrNoAddress):
		case err != nil:
			return nil, language, transient
		case address != "":
			found = append(found, notifications.Target{Channel: d.Channel, Address: address})
		}
	}
	return found, language, nil
}

// uniqueTargets keeps the channel's non-empty targets once each.
func uniqueTargets(channel notifications.Channel, in []notifications.Target) []notifications.Target {
	out := make([]notifications.Target, 0, len(in))
	seen := map[string]bool{}
	for _, t := range in {
		if t.Channel != channel || strings.TrimSpace(t.Address) == "" || seen[t.Key()] {
			continue
		}
		seen[t.Key()] = true
		out = append(out, t)
	}
	return out
}
