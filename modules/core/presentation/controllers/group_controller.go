package controllers

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
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
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
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

func (ru *GroupRealtimeUpdates) onGroupCreated(event *group.CreatedEvent) {
	logger := configuration.Use().Logger()

	updatedGroup := event.Group
	component := groups.GroupCreatedEvent(mappers.GroupToViewModel(updatedGroup), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render group created event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send group created event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast group created event to websocket")
		return
	}
}

func (ru *GroupRealtimeUpdates) onGroupDeleted(event *group.DeletedEvent) {
	logger := configuration.Use().Logger()

	component := groups.GroupRow(mappers.GroupToViewModel(event.Group), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render group deleted event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send group deleted event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast group deleted event to websocket")
		return
	}
}

func (ru *GroupRealtimeUpdates) onGroupUpdated(event *group.UpdatedEvent) {
	logger := configuration.Use().Logger()

	component := groups.GroupRow(mappers.GroupToViewModel(event.Group), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render group updated event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send group updated event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast group updated event to websocket")
		return
	}
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
	groupQueryService *services.GroupQueryService,
) {
	params := composables.UsePaginated(r)
	search := r.URL.Query().Get("name")

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving tenant from request: %v", err)
		http.Error(w, "Error retrieving tenant", http.StatusBadRequest)
		return
	}

	// Build query parameters
	findParams := &query.GroupFindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Search: search,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{Field: query.GroupFieldCreatedAt, Ascending: false},
			},
		},
		Filters: []query.GroupFilter{
			{
				Column: query.GroupFieldTenantID,
				Filter: repo.Eq(tenantID.String()),
			},
		},
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		findParams.Filters = append(findParams.Filters, query.GroupFilter{
			Column: query.GroupFieldCreatedAt,
			Filter: repo.Lt(v),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.From"); v != "" {
		findParams.Filters = append(findParams.Filters, query.GroupFilter{
			Column: query.GroupFieldCreatedAt,
			Filter: repo.Gt(v),
		})
	}

	// Use the appropriate method based on whether we're searching
	var groupViewModels []*viewmodels.Group
	var total int

	if search != "" {
		groupViewModels, total, err = groupQueryService.SearchGroups(r.Context(), findParams)
	} else {
		groupViewModels, total, err = groupQueryService.FindGroups(r.Context(), findParams)
	}

	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	isHxRequest := htmx.IsHxRequest(r)

	pageProps := &groups.IndexPageProps{
		Groups:  groupViewModels, // Already viewmodels from query service
		Page:    params.Page,
		PerPage: params.Limit,
		Search:  search,
		HasMore: total > params.Page*params.Limit,
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
	groupQueryService *services.GroupQueryService,
	roleService *services.RoleService,
) {
	idStr := mux.Vars(r)["id"]

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving tenant from request: %v", err)
		http.Error(w, "Error retrieving tenant", http.StatusBadRequest)
		return
	}

	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	// For security, we should filter by tenant even in FindGroupByID
	// We can use FindGroups with an ID filter instead
	findParams := &query.GroupFindParams{
		Limit:  1,
		Offset: 0,
		Filters: []query.GroupFilter{
			{
				Column: query.GroupFieldTenantID,
				Filter: repo.Eq(tenantID.String()),
			},
			{
				Column: query.GroupFieldID,
				Filter: repo.Eq(idStr),
			},
		},
	}

	foundGroups, _, err := groupQueryService.FindGroups(r.Context(), findParams)
	if err != nil {
		logger.Errorf("Error retrieving group: %v", err)
		http.Error(w, "Error retrieving group", http.StatusInternalServerError)
		return
	}

	if len(foundGroups) == 0 {
		logger.Errorf("Group not found or access denied: %s", idStr)
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	groupViewModel := foundGroups[0]

	props := &groups.EditFormProps{
		Group:  groupViewModel,
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

	// Get tenant from context and set it on the group
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.Errorf("Error getting tenant: %v", err)
		http.Error(w, "Error getting tenant", http.StatusInternalServerError)
		return
	}
	groupEntity = groupEntity.SetTenantID(tenantID)

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
