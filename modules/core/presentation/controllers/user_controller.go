// Package controllers provides this package.
package controllers

import (
	"bytes"
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/slot"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
)

// UserRealtimeUpdates broadcasts user CRUD events to authenticated websocket
// connections so that open user listings receive live row updates. It is
// wired via composition.ContributeEventHandlerFunc — one subscription per
// event kind — so teardown and tenant isolation stay consistent with the
// rest of the core event-handler stack. The per-controller Register/Subscribe
// dance that leaked subscribers on every router rebuild has been removed.
type UserRealtimeUpdates struct {
	app    application.Application
	logger *logrus.Logger
}

// NewUserRealtimeUpdates is the reflection-injector-friendly constructor.
func NewUserRealtimeUpdates(app application.Application, logger *logrus.Logger) *UserRealtimeUpdates {
	return &UserRealtimeUpdates{
		app:    app,
		logger: logger,
	}
}

// OnUserCreated renders the newly-created user as a table row and broadcasts
// it to every authenticated websocket client.
func (ru *UserRealtimeUpdates) OnUserCreated(event *user.CreatedEvent) {
	component := users.UserCreatedEvent(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			ru.logger.WithError(err).Error("failed to render user created event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			ru.logger.WithError(err).Error("failed to send user created event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		ru.logger.WithError(err).Error("failed to broadcast user created event to websocket")
		return
	}
}

// OnUserDeleted broadcasts a row-deletion to every authenticated websocket
// client so that open user listings remove the row.
func (ru *UserRealtimeUpdates) OnUserDeleted(event *user.DeletedEvent) {
	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})

	err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			ru.logger.WithError(err).Error("failed to render user deleted event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			ru.logger.WithError(err).Error("failed to send user deleted event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	})
	if err != nil {
		ru.logger.WithError(err).Error("failed to broadcast user deleted event to websocket")
		return
	}
}

// OnUserUpdated broadcasts the updated user row to every authenticated
// websocket client so that open user listings reflect the change.
func (ru *UserRealtimeUpdates) OnUserUpdated(event *user.UpdatedEvent) {
	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			ru.logger.WithError(err).Error("failed to render user updated event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			ru.logger.WithError(err).Error("failed to send user updated event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		ru.logger.WithError(err).Error("failed to broadcast user updated event to websocket")
		return
	}
}

type SingleSlotFunc func(ctx context.Context, user user.User, slots slot.Manager)

type UsersController struct {
	basePath             string
	permissionSchema     *rbac.PermissionSchema
	configureSingleSlots SingleSlotFunc
}

type userControllerOptions struct {
	basePath             string
	permissionSchema     *rbac.PermissionSchema
	configureSingleSlots SingleSlotFunc
}

type UserControllerOption func(*userControllerOptions)

func WithUserControllerBasePath(basePath string) UserControllerOption {
	return func(uco *userControllerOptions) {
		uco.basePath = basePath
	}
}

func WithUserControllerPermissionSchema(schema *rbac.PermissionSchema) UserControllerOption {
	return func(uco *userControllerOptions) {
		uco.permissionSchema = schema
	}
}

func WithUserControllerConfigureSingleSlots(slotFunc SingleSlotFunc) UserControllerOption {
	return func(uco *userControllerOptions) {
		uco.configureSingleSlots = slotFunc
	}
}

func NewUsersController(
	_ application.Application,
	opts ...UserControllerOption,
) application.Controller {
	o := &userControllerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.permissionSchema == nil {
		panic("UsersController requires PermissionSchema in options")
	}
	if o.basePath == "" {
		panic("UsersController requires explicit BasePath in options")
	}

	return &UsersController{
		basePath:             o.basePath,
		permissionSchema:     o.permissionSchema,
		configureSingleSlots: o.configureSingleSlots,
	}
}

func (c *UsersController) Key() string {
	return c.basePath
}

func (c *UsersController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Users)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.GetSingle)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}/edit", di.H(c.GetEdit)).Methods(http.MethodGet)

	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Delete)).Methods(http.MethodDelete)
	router.HandleFunc("/{id:[0-9]+}/block/drawer", di.H(c.GetBlockDrawer)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}/block", di.H(c.BlockUser)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}/unblock", di.H(c.UnblockUser)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}/sessions", di.H(c.GetUserSessions)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}/sessions/{token}", di.H(c.RevokeUserSession)).Methods(http.MethodDelete)
}

func (c *UsersController) resourcePermissionGroups(
	selected ...permission.Permission,
) []*viewmodels.ResourcePermissionGroup {
	return BuildResourcePermissionGroups(c.permissionSchema, selected...)
}
