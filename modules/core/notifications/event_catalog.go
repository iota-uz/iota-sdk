package notifications

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// ContributeDefinitions registers a component's notification definitions
// (FromEvent) in the settings catalog when the container is built, so the
// notification settings list them and the outbox bridge delivers their events.
// A module other than core contributes its own definitions this way.
func ContributeDefinitions(builder *composition.Builder, definitions ...Definition) {
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		catalog, err := composition.Resolve[*Catalog](container)
		if err != nil {
			return nil, err
		}
		for _, definition := range definitions {
			if err := catalog.Register(definition); err != nil {
				return nil, fmt.Errorf("register notification %s: %w", definition.Key, err)
			}
		}
		return nil, nil
	})
}

// ActorRecipient is the user who made the change an event reports (the event
// initiator); a system or job context has none.
func ActorRecipient() RecipientDefinition {
	return RecipientDefinition{
		Key:  "actor",
		Name: map[string]string{"en": "Event actor", "ru": "Инициатор события", "uz": "Voqea tashabbuskori", "uz-Cyrl": "Воқеа ташаббускори"},
		Resolve: func(_ context.Context, event Event) ([]uint, error) {
			if event.ActorUserID == 0 {
				return nil, nil
			}
			return []uint{event.ActorUserID}, nil
		},
	}
}

// DataUserRecipient is the user whose ID the event carries in the Data field.
func DataUserRecipient(key string, name map[string]string, field string) RecipientDefinition {
	return RecipientDefinition{
		Key:  key,
		Name: name,
		Resolve: func(_ context.Context, event Event) ([]uint, error) {
			id, err := strconv.ParseUint(event.Data[field], 10, 32)
			if err != nil || id == 0 {
				return nil, fmt.Errorf("notification %s: invalid %s", event.Key, field)
			}
			return []uint{uint(id)}, nil
		},
	}
}

// EventContent renders the notice of an event: the localized label as the
// title, body(event) as the text (the localized description when it is empty)
// and url(event) as the link; either function may be nil.
func EventContent(label, description map[string]string, body, url func(Event) string) func(Event, string) (Content, error) {
	return func(event Event, language string) (Content, error) {
		content := Content{Title: Localized(label, language)}
		if body != nil {
			content.Body = strings.TrimSpace(body(event))
		}
		if content.Body == "" {
			content.Body = Localized(description, language)
		}
		if url != nil {
			content.ActionURL = url(event)
		}
		return content, nil
	}
}

// DataName joins the Data fields as one display name.
func DataName(event Event, fields ...string) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if v := strings.TrimSpace(event.Data[f]); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

// DataURL is a link built from a path prefix and a Data field; empty when the
// field is.
func DataURL(prefix, field string) func(Event) string {
	return func(event Event) string {
		if v := event.Data[field]; v != "" {
			return prefix + v
		}
		return ""
	}
}
