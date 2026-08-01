package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	lensshare "github.com/iota-uz/iota-sdk/pkg/lens/share"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LensShareController struct {
	handler *lensshare.HTTPHandler
}

var _ application.Controller = (*LensShareController)(nil)

func NewLensShareController(pool *pgxpool.Pool, roleService *services.RoleService, options ...lensshare.HTTPOption) (*LensShareController, error) {
	if roleService == nil {
		return nil, fmt.Errorf("role service is required")
	}
	store, err := lensshare.NewPostgresStore(pool)
	if err != nil {
		return nil, err
	}
	handler, err := lensshare.NewHTTPHandler(store, lensShareAccess(roleService), "/lens/share", options...)
	if err != nil {
		return nil, err
	}
	return &LensShareController{handler: handler}, nil
}

func (c *LensShareController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("lens.share", 0, application.Prefix("/lens/share"))
}

func (c *LensShareController) Register(router *mux.Router) {
	shareRouter := router.NewRoute().Subrouter()
	shareRouter.Use(middleware.Authorize(), middleware.ProvideUser())
	c.handler.Register(shareRouter)
}

func (c *LensShareController) ViewsEndpoint() string     { return c.handler.ViewsEndpoint() }
func (c *LensShareController) SchedulesEndpoint() string { return c.handler.SchedulesEndpoint() }

func lensShareAccess(roleService *services.RoleService) lensshare.AccessResolver {
	return func(r *http.Request) (lensshare.Access, error) {
		actor, err := composables.UseUser(r.Context())
		if err != nil {
			return lensshare.Access{}, fmt.Errorf("%w: %v", lensshare.ErrUnauthenticated, err)
		}
		tenantID, err := composables.UseTenantID(r.Context())
		if err != nil {
			return lensshare.Access{}, fmt.Errorf("%w: %v", lensshare.ErrUnauthenticated, err)
		}
		if actor.TenantID() != tenantID {
			return lensshare.Access{}, fmt.Errorf("actor tenant does not match request tenant")
		}
		roleIDs := make([]uint, 0, len(actor.Roles()))
		for _, role := range actor.Roles() {
			roleIDs = append(roleIDs, role.ID())
		}
		manageTeam := actor.Can(permissions.LensViewTeamManage)
		roles := make([]lensshare.RoleRef, 0)
		if manageTeam {
			roleEntities, queryErr := roleService.GetPaginated(r.Context(), &role.FindParams{
				Limit:   200,
				SortBy:  role.SortBy{Fields: []repo.SortByField[role.Field]{{Field: role.NameField, Ascending: true}}},
				Filters: []role.Filter{{Column: role.TenantIDField, Filter: repo.Eq(tenantID.String())}},
			})
			if queryErr != nil {
				return lensshare.Access{}, queryErr
			}
			for _, roleEntity := range roleEntities {
				roles = append(roles, lensshare.RoleRef{ID: roleEntity.ID(), Name: roleEntity.Name()})
			}
		}
		return lensshare.Access{
			TenantID: tenantID, UserID: actor.ID(), RoleIDs: roleIDs, Roles: roles,
			ManageTeam: manageTeam, ScheduleMail: actor.Can(permissions.LensExportSchedule),
		}, nil
	}
}
