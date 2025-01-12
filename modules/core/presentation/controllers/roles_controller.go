package controllers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/roles"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
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
	pageCtx, err := composables.UsePageCtx(
		r, types.NewPageData("Roles.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
		PageContext: pageCtx,
		Roles:       mapping.MapViewModels(roleEntities, mappers.RoleToViewModel),
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

	pageCtx, err := composables.UsePageCtx(
		r, types.NewPageData("Roles.Meta.Edit.Title", ""),
	)
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
		PageContext: pageCtx,
		Role:        mappers.RoleToViewModel(roleEntity),
		Errors:      map[string]string{},
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
	dto, err := composables.UseForm(&user.UpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pageCtx, err := composables.UsePageCtx(
		r, types.NewPageData("Roles.Meta.Edit.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errors, ok := dto.Ok(r.Context()); !ok {
		roleEntity, err := c.roleService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}

		props := &roles.EditFormProps{
			PageContext: pageCtx,
			Role:        mappers.RoleToViewModel(roleEntity),
			Errors:      errors,
		}
		templ.Handler(roles.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	//userEntity, err := dto.ToEntity(id)
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}
	//if err := c.roleService.Update(r.Context(), userEntity); err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}
	shared.Redirect(w, r, c.basePath)
}

func (c *RolesController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Roles.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	permissionsGroupedByResource := make(map[string][]*permission.Permission)
	for _, perm := range c.app.Permissions() {
		resource := string(perm.Resource)
		permissionsGroupedByResource[resource] = append(permissionsGroupedByResource[resource], perm)
	}
	var permissionGroups []*roles.Group
	for resource, permissions := range permissionsGroupedByResource {
		var children []*roles.Child
		for _, perm := range permissions {
			children = append(children, &roles.Child{
				Name:    perm.ID.String(),
				Label:   perm.Name,
				Checked: false,
			})
		}
		permissionGroups = append(permissionGroups, &roles.Group{
			Label:    resource,
			Children: children,
		})
	}
	props := &roles.CreateFormProps{
		PageContext:      pageCtx,
		Role:             &viewmodels.Role{},
		PermissionGroups: permissionGroups,
		Errors:           map[string]string{},
	}
	templ.Handler(roles.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *RolesController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&user.CreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Roles.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		//userEntity, err := dto.ToEntity()
		//if err != nil {
		//	http.Error(w, err.Error(), http.StatusInternalServerError)
		//	return
		//}
		roleEntity, err := role.New("", "", []*permission.Permission{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &roles.CreateFormProps{
			PageContext: pageCtx,
			Role:        mappers.RoleToViewModel(roleEntity),
			Errors:      errors,
		}
		templ.Handler(
			roles.CreateForm(props), templ.WithStreaming(),
		).ServeHTTP(w, r)
		return
	}
	//
	//userEntity, err := dto.ToEntity()
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}
	//if err := c.roleService.Create(r.Context(), userEntity); err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}

	shared.Redirect(w, r, c.basePath)
}
