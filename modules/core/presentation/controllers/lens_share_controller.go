package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	lensshare "github.com/iota-uz/iota-sdk/pkg/lens/share"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LensShareController struct {
	handler *lensshare.HTTPHandler
}

var _ application.Controller = (*LensShareController)(nil)

func NewLensShareController(pool *pgxpool.Pool, options ...lensshare.HTTPOption) (*LensShareController, error) {
	store, err := lensshare.NewPostgresStore(pool)
	if err != nil {
		return nil, err
	}
	handler, err := lensshare.NewHTTPHandler(store, lensShareAccess(pool), "/lens/share", options...)
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

func lensShareAccess(pool *pgxpool.Pool) lensshare.AccessResolver {
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
			rows, queryErr := pool.Query(r.Context(), `SELECT id, name FROM roles WHERE tenant_id = $1 ORDER BY lower(name), id`, tenantID)
			if queryErr != nil {
				return lensshare.Access{}, queryErr
			}
			defer rows.Close()
			for rows.Next() {
				var role lensshare.RoleRef
				if scanErr := rows.Scan(&role.ID, &role.Name); scanErr != nil {
					return lensshare.Access{}, scanErr
				}
				roles = append(roles, role)
			}
			if rows.Err() != nil {
				return lensshare.Access{}, rows.Err()
			}
		}
		return lensshare.Access{
			TenantID: tenantID, UserID: actor.ID(), RoleIDs: roleIDs, Roles: roles,
			ManageTeam: manageTeam, ScheduleMail: actor.Can(permissions.LensExportSchedule),
		}, nil
	}
}
