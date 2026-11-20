package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// RunAs selects who a Go consumer acts as for an event.
type RunAs string

const (
	// RunAsInitiator acts as the event's initiator with the initiator's
	// current rights; an event without an initiator acts as the trigger
	// owner.
	RunAsInitiator RunAs = "initiator"
	// RunAsOwner always acts as the trigger owner.
	RunAsOwner RunAs = "owner"
)

// ErrNoSubject is matched by a refused subject: the event has no initiator
// (or the mode ignores it) and the trigger has no owner in the tenant, or the
// selected user is blocked or deleted. A Consumer.Deliver error that wraps it
// blocks the delivery instead of failing it: the dispatcher re-evaluates it
// every BlockedRecheck without counting attempts, so assigning an owner or
// enabling the user resumes delivery.
var ErrNoSubject = errors.New("extruntime events: no subject")

// ResolveSubject returns the subject a consumer acts as for ev: the initiator
// (RunAsInitiator, when the event has one) or the owner of triggerID in the
// event's tenant (extruntime.trigger_owners), with current rights from
// subjects. q is the delivery transaction or a pool.
func ResolveSubject(ctx context.Context, q triggerowner.Querier, subjects host.SubjectLoader, ev Envelope, triggerID string, mode RunAs) (spi.Subject, error) {
	const op = serrors.Op("extruntime.events.ResolveSubject")
	if subjects == nil {
		return spi.Subject{}, serrors.New(serrors.Invalid, "extruntime events: no subject loader").WithOp(op)
	}
	var userID int64
	kind := spi.SubjectInitiator
	switch {
	case mode == RunAsInitiator && ev.InitiatorUserID != nil:
		userID = *ev.InitiatorUserID
	case mode == RunAsInitiator || mode == RunAsOwner:
		kind = spi.SubjectOwner
		owner, found, err := triggerowner.Lookup(ctx, q, triggerID, ev.TenantID)
		if err != nil {
			return spi.Subject{}, serrors.Wrap(op, err)
		}
		if !found {
			if mode == RunAsOwner {
				return spi.Subject{}, fmt.Errorf("%w: trigger %s has no owner-user in tenant %s", ErrNoSubject, triggerID, ev.TenantID)
			}
			return spi.Subject{}, fmt.Errorf("%w: event %s has no initiator and trigger %s has no owner-user in tenant %s", ErrNoSubject, ev.ID, triggerID, ev.TenantID)
		}
		userID = owner
	default:
		return spi.Subject{}, serrors.New(serrors.Invalid, fmt.Sprintf("extruntime events: unknown run-as mode %q", mode)).WithOp(op)
	}
	subject, err := subjects.LoadSubject(ctx, ev.TenantID, userID, kind)
	if errors.Is(err, host.ErrSubjectBlocked) {
		return spi.Subject{}, fmt.Errorf("%w: %w", ErrNoSubject, err)
	}
	if err != nil {
		return spi.Subject{}, serrors.Wrap(op, err)
	}
	subject.TenantID, subject.UserID, subject.Kind = ev.TenantID, userID, kind
	return subject, nil
}
