package controllers

import (
	"bytes"
	"context"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// UserRealtimeUpdates broadcasts user CRUD events to authenticated websocket
// connections so that open user listings receive live row updates.
type UserRealtimeUpdates struct {
	app application.Application
}

func NewUserRealtimeUpdates(app application.Application) *UserRealtimeUpdates {
	return &UserRealtimeUpdates{app: app}
}

func (ru *UserRealtimeUpdates) broadcastToTenant(tenantID uuid.UUID, component templ.Component, label string) {
	logger := configuration.Use().Logger()

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		if conn.User().TenantID() != tenantID {
			return nil
		}
		if !conn.User().Can(permissions.UserRead) {
			return nil
		}

		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Errorf("failed to render %s event for websocket", label)
			return nil
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Errorf("failed to send %s event to websocket connection", label)
			return nil
		}
		return nil
	}); err != nil {
		logger.WithError(err).Errorf("failed to broadcast %s event to websocket", label)
	}
}

func (ru *UserRealtimeUpdates) OnUserCreated(event *user.CreatedEvent) {
	component := users.UserCreatedEvent(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})
	ru.broadcastToTenant(event.Result.TenantID(), component, "user created")
}

func (ru *UserRealtimeUpdates) OnUserDeleted(event *user.DeletedEvent) {
	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})
	ru.broadcastToTenant(event.Result.TenantID(), component, "user deleted")
}

func (ru *UserRealtimeUpdates) OnUserUpdated(event *user.UpdatedEvent) {
	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})
	ru.broadcastToTenant(event.Result.TenantID(), component, "user updated")
}
