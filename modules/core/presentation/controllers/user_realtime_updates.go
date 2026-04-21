package controllers

import (
	"bytes"
	"context"

	"github.com/a-h/templ"

	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
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

func (ru *UserRealtimeUpdates) OnUserCreated(event *user.CreatedEvent) {
	logger := configuration.Use().Logger()

	component := users.UserCreatedEvent(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		if conn.User().TenantID() != event.Result.TenantID() {
			return nil
		}

		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render user created event for websocket")
			return nil
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send user created event to websocket connection")
			return nil
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast user created event to websocket")
	}
}

func (ru *UserRealtimeUpdates) OnUserDeleted(event *user.DeletedEvent) {
	logger := configuration.Use().Logger()

	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		if conn.User().TenantID() != event.Result.TenantID() {
			return nil
		}

		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render user deleted event for websocket")
			return nil
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send user deleted event to websocket connection")
			return nil
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast user deleted event to websocket")
	}
}

func (ru *UserRealtimeUpdates) OnUserUpdated(event *user.UpdatedEvent) {
	logger := configuration.Use().Logger()

	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		if conn.User().TenantID() != event.Result.TenantID() {
			return nil
		}

		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render user updated event for websocket")
			return nil
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send user updated event to websocket connection")
			return nil
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast user updated event to websocket")
	}
}
