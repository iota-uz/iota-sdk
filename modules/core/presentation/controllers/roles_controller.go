package controllers

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/roles"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"net/http"
)

type RolesController struct {
	app         application.Application
	roleService *services.RoleService
	basePath    string
}

func NewRolesController(app application.Application) application.Controller {
	return &RolesController{
		app:         app,
		roleService: app.Service(services.RoleService{}).(*services.RoleService),
		basePath:    "/roles",
	}
}

func (c *RolesController) Key() string {
	return c.basePath
}

func (c *RolesController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *RolesController) List(w http.ResponseWriter, r *http.Request) {
	params := composables.UsePaginated(r)
	roleEntities, err := c.roleService.GetPaginated(r.Context(), &role.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &roles.IndexPageProps{
		Roles: mapping.MapViewModels(roleEntities, mappers.RoleToViewModel),
	}
	if isHxRequest {
		templ.Handler(roles.RolesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(roles.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *RolesController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roleEntity, err := c.roleService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}
	props := &roles.EditFormProps{
		Role:             mappers.RoleToViewModel(roleEntity),
		PermissionGroups: c.permissionGroups(roleEntity.Permissions()...),
		Errors:           map[string]string{},
	}
	templ.Handler(roles.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *RolesController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := c.roleService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *RolesController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto, err := composables.UseForm(&dtos.UpdateRoleDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	roleEntity, err := c.roleService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}
	if errors, ok := dto.Ok(r.Context()); !ok {
		props := &roles.EditFormProps{
			Role:             mappers.RoleToViewModel(roleEntity),
			PermissionGroups: c.permissionGroups(roleEntity.Permissions()...),
			Errors:           errors,
		}
		templ.Handler(roles.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	updatedEntity, err := dto.ToEntity(roleEntity, c.app.RBAC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := c.roleService.Update(r.Context(), updatedEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *RolesController) permissionGroups(selected ...*permission.Permission) []*roles.Group {
	isSelected := func(p2 *permission.Permission) bool {
		for _, p1 := range selected {
			if p1.ID == p2.ID {
				return true
			}
		}
		return false
	}
	groupedByResource := make(map[string][]*permission.Permission)
	for _, perm := range c.app.RBAC().Permissions() {
		resource := string(perm.Resource)
		groupedByResource[resource] = append(groupedByResource[resource], perm)
	}
	groups := make([]*roles.Group, 0, len(groupedByResource))
	for resource, permissions := range groupedByResource {
		var children []*roles.Child
		for _, perm := range permissions {
			children = append(children, &roles.Child{
				Name:    fmt.Sprintf("Permissions[%s]", perm.ID.String()),
				Label:   perm.Name,
				Checked: isSelected(perm),
			})
		}
		groups = append(groups, &roles.Group{
			Label:    resource,
			Children: children,
		})
	}
	return groups
}

func (c *RolesController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &roles.CreateFormProps{
		Role:             &viewmodels.Role{},
		PermissionGroups: c.permissionGroups(),
		Errors:           map[string]string{},
	}
	templ.Handler(roles.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *RolesController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.CreateRoleDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		roleEntity, err := dto.ToEntity(c.app.RBAC())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &roles.CreateFormProps{
			Role:             mappers.RoleToViewModel(roleEntity),
			PermissionGroups: c.permissionGroups(),
			Errors:           errors,
		}
		templ.Handler(roles.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	roleEntity, err := dto.ToEntity(c.app.RBAC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := c.roleService.Create(r.Context(), roleEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
