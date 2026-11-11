package services

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// UserCreatedEventID is the versioned ID of the business event published when
// a user is created. Extension packages subscribe to it by this ID.
const UserCreatedEventID = "core.user.created.v1"

// UserCreatedPayload is the versioned payload of core.user.created.v1.
type UserCreatedPayload struct {
	UserID    int64  `json:"userId,string" wire:"int64"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	UserType  string `json:"userType"`
}

// UserCreatedEvent declares the event and its versioned payload. The core
// module contributes it to the application's native set (Natives), next to
// the events declared by extension packages.
var UserCreatedEvent = native.Event[UserCreatedPayload](UserCreatedEventID, native.EventSpec{
	Subject:     "core.user",
	Description: "A user account was created. Published in the transaction that creates the user.",
})

// Natives is the core module's native registration: the core.user entity and
// the core.user.created.v1 event. Applications compose it into their
// native.Set (modules.Natives).
func Natives(s *native.Set) error {
	return s.Add(native.Entity(data.CoreUser()), UserCreatedEvent)
}

// publishUserCreated enqueues core.user.created.v1 in the transaction held by
// ctx, so the event commits or rolls back with the user. The event belongs to
// the created user's tenant; the acting user, when there is one, is the event
// initiator.
func publishUserCreated(ctx context.Context, created user.User) error {
	const op = serrors.Op("UserService.publishUserCreated")
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tenantID := created.TenantID()
	if tenantID == uuid.Nil {
		if tenantID, err = composables.UseTenantID(ctx); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	var initiator int64
	if actor, actorErr := composables.UseUser(ctx); actorErr == nil && actor != nil {
		initiator = int64(actor.ID())
	}
	_, err = events.Publish(ctx, tx,
		spi.Scope{Subject: spi.Subject{TenantID: tenantID, UserID: initiator, Kind: spi.SubjectInitiator}},
		UserCreatedEvent,
		&spi.ObjectRef{Entity: "core.user", Key: strconv.FormatUint(uint64(created.ID()), 10)},
		UserCreatedPayload{
			UserID:    int64(created.ID()),
			Email:     created.Email().Value(),
			FirstName: created.FirstName(),
			LastName:  created.LastName(),
			UserType:  string(created.Type()),
		})
	return serrors.Wrap(op, err)
}
