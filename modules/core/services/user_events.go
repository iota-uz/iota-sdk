package services

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// UserCreatedEventID is the versioned ID of the business event published when
// a user is created. Extension packages subscribe to it by this ID.
const UserCreatedEventID = "core.user.created.v1"

// UserCreatedEvent declares the event and its versioned payload in the native
// event catalog, next to the events declared by extension packages.
var UserCreatedEvent = events.MustRegisterNative(events.NativeEvent{
	ID:          UserCreatedEventID,
	Subject:     "core.user",
	Description: "A user account was created. Published in the transaction that creates the user.",
	Payload: manifest.Type{
		Kind: manifest.KindObject,
		Properties: []manifest.Property{
			{Name: "userId", Type: manifest.Type{Kind: manifest.KindInt64}},
			{Name: "email", Type: manifest.Type{Kind: manifest.KindString}},
			{Name: "firstName", Type: manifest.Type{Kind: manifest.KindString}},
			{Name: "lastName", Type: manifest.Type{Kind: manifest.KindString}},
			{Name: "userType", Type: manifest.Type{Kind: manifest.KindString}},
		},
	},
})

type userCreatedPayload struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	UserType  string `json:"userType"`
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
	payload, err := json.Marshal(userCreatedPayload{
		UserID:    strconv.FormatUint(uint64(created.ID()), 10),
		Email:     created.Email().Value(),
		FirstName: created.FirstName(),
		LastName:  created.LastName(),
		UserType:  string(created.Type()),
	})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = events.Publish(ctx, tx,
		spi.Scope{Subject: spi.Subject{TenantID: tenantID, UserID: initiator, Kind: spi.SubjectInitiator}},
		spi.EventIntent{
			Event:   UserCreatedEventID,
			Subject: &spi.ObjectRef{Entity: "core.user", Key: strconv.FormatUint(uint64(created.ID()), 10)},
			Payload: payload,
		})
	return serrors.Wrap(op, err)
}
