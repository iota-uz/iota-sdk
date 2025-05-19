package controllers

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/groups"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

type GroupRealtimeUpdates struct {
	app          application.Application
	groupService *services.GroupService
	basePath     string
}

func NewGroupRealtimeUpdates(app application.Application, groupService *services.GroupService, basePath string) *GroupRealtimeUpdates {
	return &GroupRealtimeUpdates{
		app:          app,
		groupService: groupService,
		basePath:     basePath,
	}
}

func (ru *GroupRealtimeUpdates) Register() {
	ru.app.EventPublisher().Subscribe(ru.onGroupCreated)
	ru.app.EventPublisher().Subscribe(ru.onGroupUpdated)
	ru.app.EventPublisher().Subscribe(ru.onGroupDeleted)
}

func (ru *GroupRealtimeUpdates) publisherContext() (context.Context, error) {
	localizer := i18n.NewLocalizer(ru.app.Bundle(), "en")
	ctx := intl.WithLocalizer(
		context.Background(),
		localizer,
	)
	_url, err := url.Parse(ru.basePath)
	if err != nil {
		return nil, err
	}
	ctx = composables.WithPageCtx(ctx, &types.PageContext{
		URL:       _url,
		Locale:    language.English,
		Localizer: localizer,
	})
	return composables.WithPool(ctx, ru.app.DB()), nil
}

func (ru *GroupRealtimeUpdates) onGroupCreated(event *group.CreatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	updatedGroup := event.Group
	component := groups.GroupCreatedEvent(mappers.GroupToViewModel(updatedGroup), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering group row: %v | Event: onGroupCreated", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

func (ru *GroupRealtimeUpdates) onGroupDeleted(event *group.DeletedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	component := groups.GroupRow(mappers.GroupToViewModel(event.Group), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering group row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

func (ru *GroupRealtimeUpdates) onGroupUpdated(event *group.UpdatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	component := groups.GroupRow(mappers.GroupToViewModel(event.Group), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering group row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

type GroupsController struct {
	app      application.Application
	basePath string
	realtime *GroupRealtimeUpdates
}

func NewGroupsController(app application.Application) application.Controller {
	groupService := app.Service(services.GroupService{}).(*services.GroupService)
	basePath := "/groups"

	controller := &GroupsController{
		app:      app,
		basePath: basePath,
		realtime: NewGroupRealtimeUpdates(app, groupService, basePath),
	}

	return controller
}

func (c *GroupsController) Key() string {
	return c.basePath
}

func (c *GroupsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Groups)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[a-f0-9-]+}", di.H(c.GetEdit)).Methods(http.MethodGet)

	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[a-f0-9-]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[a-f0-9-]+}", di.H(c.Delete)).Methods(http.MethodDelete)

	c.realtime.Register()
}

func (c *GroupsController) Groups(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	groupService *services.GroupService,
) {
	params := composables.UsePaginated(r)
	search := r.URL.Query().Get("name")

	tenant, err := composables.UseTenant(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving tenant from request context: %v", err)
		http.Error(w, "Error retrieving tenant", http.StatusBadRequest)
		return
	}

	findParams := &group.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: group.SortBy{Fields: []group.Field{}},
		Search: search,
		Filters: []group.Filter{
			{
				// TODO: come back to do this
				Column: group.TenantIDField,
				Filter: repo.Eq(tenant.ID.String()),
			},
		},
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		findParams.Filters = append(findParams.Filters, group.Filter{
			Column: group.CreatedAtField,
			Filter: repo.Lt(v),
		})
	}

	groupEntities, total, err := groupService.GetPaginatedWithTotal(r.Context(), findParams)

	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}
	isHxRequest := htmx.IsHxRequest(r)

	viewModelGroups := mapping.MapViewModels(groupEntities, mappers.GroupToViewModel)

	pageProps := &groups.IndexPageProps{
		Groups:  viewModelGroups,
		Page:    params.Page,
		PerPage: params.Limit,
		Search:  search,
		HasMore: total > int64(params.Page*params.Limit),
	}

	if isHxRequest {
		if params.Page > 1 {
			templ.Handler(groups.GroupRows(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			templ.Handler(groups.GroupsTable(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
		}
	} else {
		templ.Handler(groups.Index(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *GroupsController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	groupService *services.GroupService,
	roleService *services.RoleService,
) {
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		logger.Errorf("Error parsing group ID: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	groupEntity, err := groupService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving group: %v", err)
		http.Error(w, "Error retrieving group", http.StatusInternalServerError)
		return
	}

	props := &groups.EditFormProps{
		Group:  mappers.GroupToViewModel(groupEntity),
		Roles:  mapping.MapViewModels(roles, mappers.RoleToViewModel),
		Errors: map[string]string{},
	}

	templ.Handler(groups.EditGroupDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *GroupsController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
) {
	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	props := &groups.CreateFormProps{
		Group:  &groups.GroupFormData{},
		Roles:  mapping.MapViewModels(roles, mappers.RoleToViewModel),
		Errors: map[string]string{},
	}
	templ.Handler(groups.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *GroupsController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	groupService *services.GroupService,
	roleService *services.RoleService,
) {
	dto, err := composables.UseForm(&dtos.CreateGroupDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		roles, err := roleService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving roles: %v", err)
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}

		props := &groups.CreateFormProps{
			Group: &groups.GroupFormData{
				Name:        dto.Name,
				Description: dto.Description,
				RoleIDs:     dto.RoleIDs,
			},
			Roles:  mapping.MapViewModels(roles, mappers.RoleToViewModel),
			Errors: errors,
		}
		templ.Handler(
			groups.CreateForm(props), templ.WithStreaming(),
		).ServeHTTP(w, r)
		return
	}

	groupEntity, err := dto.ToEntity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Process role assignments
	for _, roleIDStr := range dto.RoleIDs {
		roleID, err := strconv.ParseUint(roleIDStr, 10, 64)
		if err != nil {
			continue
		}
		role, err := roleService.GetByID(r.Context(), uint(roleID))
		if err != nil {
			continue
		}
		groupEntity = groupEntity.AssignRole(role)
	}

	if _, err := groupService.Create(r.Context(), groupEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		form := r.FormValue("form")
		if form == "drawer-form" {
			htmx.SetTrigger(w, "closeDrawer", `{"id": "new-group-drawer"}`)
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *GroupsController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	groupService *services.GroupService,
	roleService *services.RoleService,
) {
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&dtos.UpdateGroupDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		roles, err := roleService.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}

		props := &groups.EditFormProps{
			Group: &viewmodels.Group{
				ID:          id.String(),
				Name:        dto.Name,
				Description: dto.Description,
			},
			Roles:  mapping.MapViewModels(roles, mappers.RoleToViewModel),
			Errors: errors,
		}

		templ.Handler(groups.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	existingGroup, err := groupService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving group: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	roles := make([]role.Role, 0, len(dto.RoleIDs))

	for _, rID := range dto.RoleIDs {
		rUintID, err := strconv.ParseUint(rID, 10, 64)
		if err != nil {
			logger.Errorf("Error parsing role id: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		role, err := roleService.GetByID(r.Context(), uint(rUintID))
		if err != nil {
			logger.Errorf("Error getting role: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		roles = append(roles, role)
	}

	groupEntity, err := dto.Apply(existingGroup, roles)
	if err != nil {
		logger.Errorf("Error updating group: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := groupService.Update(r.Context(), groupEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		htmx.SetTrigger(w, "closeDrawer", `{"id": "edit-group-drawer"}`)
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *GroupsController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	groupService *services.GroupService,
) {
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := groupService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}
