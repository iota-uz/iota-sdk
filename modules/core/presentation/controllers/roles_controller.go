package controllers

import (
	"net/http"
	"sort"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/roles"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
)

type RolesController struct {
	app      application.Application
	basePath string
}

func NewRolesController(app application.Application) application.Controller {
	return &RolesController{
		app:      app,
		basePath: "/roles",
	}
}

func (c *RolesController) Key() string {
	return c.basePath
}

func (c *RolesController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.GetEdit)).Methods(http.MethodGet)

	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Delete)).Methods(http.MethodDelete)
}

func (c *RolesController) permissionGroups(
	rbac rbac.RBAC,
	selected ...*permission.Permission,
) []*viewmodels.PermissionGroup {
	isSelected := func(p2 *permission.Permission) bool {
		for _, p1 := range selected {
			if p1.ID == p2.ID {
				return true
			}
		}
		return false
	}

	// Use the PermissionsByResource method
	groupedByResource := rbac.PermissionsByResource()

	groups := make([]*viewmodels.PermissionGroup, 0, len(groupedByResource))
	for resource, permissions := range groupedByResource {
		var permList []*viewmodels.PermissionItem
		for _, perm := range permissions {
			permList = append(permList, &viewmodels.PermissionItem{
				ID:      perm.ID.String(),
				Name:    perm.Name,
				Checked: isSelected(perm),
			})
		}
		groups = append(groups, &viewmodels.PermissionGroup{
			Resource:    resource,
			Permissions: permList,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Resource < groups[j].Resource
	})
	return groups
}

func (c *RolesController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
) {
	params := composables.UsePaginated(r)
	search := r.URL.Query().Get("name")

	tenant, err := composables.UseTenant(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving tenant from request context: %v", err)
		http.Error(w, "Error retrieving tenant", http.StatusBadRequest)
		return
	}

	// Create find params with search
	findParams := &role.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Filters: []role.Filter{
			{
				Column: role.TenantID,
				Filter: repo.Eq(tenant.ID.String()),
			},
		},
	}

	// Apply search filter if provided
	if search != "" {
		findParams.Search = search
	}

	roleEntities, err := roleService.GetPaginated(r.Context(), findParams)
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	props := &roles.IndexPageProps{
		Roles:  mapping.MapViewModels(roleEntities, mappers.RoleToViewModel),
		Search: search,
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(roles.RoleRows(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(roles.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *RolesController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing role ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roleEntity, err := roleService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving role: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}
	props := &roles.EditFormProps{
		Role:             mappers.RoleToViewModel(roleEntity),
		PermissionGroups: c.permissionGroups(c.app.RBAC(), roleEntity.Permissions()...),
		Errors:           map[string]string{},
	}
	templ.Handler(roles.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *RolesController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing role ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := roleService.Delete(r.Context(), id); err != nil {
		logger.Errorf("Error deleting role: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *RolesController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing role ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.UpdateRoleDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	roleEntity, err := roleService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving role: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		props := &roles.EditFormProps{
			Role:             mappers.RoleToViewModel(roleEntity),
			PermissionGroups: c.permissionGroups(c.app.RBAC(), roleEntity.Permissions()...),
			Errors:           errors,
		}
		templ.Handler(roles.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	updatedEntity, err := dto.Apply(roleEntity, c.app.RBAC())
	if err != nil {
		logger.Errorf("Error updating role entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := roleService.Update(r.Context(), updatedEntity); err != nil {
		logger.Errorf("Error updating role: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *RolesController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := &roles.CreateFormProps{
		Role:             &viewmodels.Role{},
		PermissionGroups: c.permissionGroups(c.app.RBAC()),
		Errors:           map[string]string{},
	}
	templ.Handler(roles.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *RolesController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
) {
	dto, err := composables.UseForm(&dtos.CreateRoleDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		roleEntity, err := dto.ToEntity(c.app.RBAC())
		if err != nil {
			logger.Errorf("Error converting DTO to entity: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &roles.CreateFormProps{
			Role:             mappers.RoleToViewModel(roleEntity),
			PermissionGroups: c.permissionGroups(c.app.RBAC()),
			Errors:           errors,
		}
		templ.Handler(roles.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	roleEntity, err := dto.ToEntity(c.app.RBAC())
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := roleService.Create(r.Context(), roleEntity); err != nil {
		logger.Errorf("Error creating role: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
