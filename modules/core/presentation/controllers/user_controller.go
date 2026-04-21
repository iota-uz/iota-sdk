// Package controllers provides this package.
package controllers

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/base/slot"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
)

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
