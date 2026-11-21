package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
)

// UserCreatedNotificationEvent is the key of the user-created notification:
// the ID of the core.user.created.v1 event it is built from.
const UserCreatedNotificationEvent = services.UserCreatedEventID

// LegacyUserCreatedNotificationPrefix starts the ID of the user-created
// notifications the removed eventbus handler queued ("user-created:<user ID>").
const LegacyUserCreatedNotificationPrefix = "user-created:"

func newCoreNotificationCatalog(extra []notifications.Definition) (*notifications.Catalog, error) {
	catalog := notifications.NewCatalog()
	definitions := append([]notifications.Definition{notifications.TestDefinition(), userCreatedNotification()}, extra...)
	for _, definition := range definitions {
		if err := catalog.Register(definition); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

// userCreatedNotification delivers core.user.created.v1 from the outbox
// (services.NewNotificationOutboxBridge), in the transaction of the event's
// delivery.
func userCreatedNotification() notifications.Definition {
	recipientName := func(en, ru, uz, uzCyrl, pt, zh string) map[string]string {
		return map[string]string{"en": en, "ru": ru, "uz": uz, "uz-Cyrl": uzCyrl, "pt-BR": pt, "zh": zh}
	}
	return notifications.FromEvent(services.UserCreatedEvent,
		notifications.WithModule("core"),
		notifications.WithRequiredPermission(permissions.UserRead),
		notifications.WithLevel(notification.LevelSuccess),
		notifications.WithData(userCreatedData),
		// Jobs queued by the eventbus handler before the outbox bridge carry
		// user_id and name in Data and no payload; they still deliver.
		notifications.WithLegacyEvents(func(event notifications.Event) bool {
			return strings.HasPrefix(event.ID, LegacyUserCreatedNotificationPrefix)
		}),
		notifications.WithPayloadFields(
			notifications.PayloadField{Key: "user_id", Required: true},
			notifications.PayloadField{Key: "name", Required: true},
		),
		notifications.WithRecipients(
			notifications.RecipientDefinition{
				Key:  "actor",
				Name: recipientName("Event actor", "Инициатор события", "Voqea tashabbuskori", "Воқеа ташаббускори", "Responsável pelo evento", "事件发起人"),
				Resolve: func(_ context.Context, event notifications.Event) ([]uint, error) {
					if event.ActorUserID == 0 {
						return nil, nil
					}
					return []uint{event.ActorUserID}, nil
				},
			},
			notifications.RecipientDefinition{
				Key:  "created_user",
				Name: recipientName("Created user", "Созданный пользователь", "Yaratilgan foydalanuvchi", "Яратилган фойдаланувчи", "Usuário criado", "新建用户"),
				Resolve: func(_ context.Context, event notifications.Event) ([]uint, error) {
					id, err := strconv.ParseUint(event.Data["user_id"], 10, 32)
					if err != nil {
						return nil, err
					}
					return []uint{uint(id)}, nil
				},
			},
		),
		notifications.WithRender(func(event notifications.Event, language string) (notifications.Content, error) {
			id, err := strconv.ParseUint(event.Data["user_id"], 10, 64)
			if err != nil || id == 0 || event.Data["name"] == "" {
				return notifications.Content{}, fmt.Errorf("invalid user-created notification payload")
			}
			return notifications.Content{Title: notifications.Localized(services.UserCreatedEvent.Label(), language), Body: event.Data["name"], ActionURL: fmt.Sprintf("/users/%d", id)}, nil
		}),
	)
}

// userCreatedData keeps the Data the user-created rules and renderer read:
// user_id and the display name (#ID for an account without a name).
func userCreatedData(payload json.RawMessage) (map[string]string, error) {
	var p services.UserCreatedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode %s payload: %w", services.UserCreatedEventID, err)
	}
	if p.UserID <= 0 || p.UserID > 1<<32-1 {
		return nil, fmt.Errorf("invalid user-created user_id")
	}
	name := strings.TrimSpace(p.FirstName + " " + p.LastName)
	if name == "" {
		name = fmt.Sprintf("#%d", p.UserID)
	}
	return map[string]string{"user_id": strconv.FormatInt(p.UserID, 10), "name": name}, nil
}
