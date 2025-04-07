package controllers

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/users"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
)

type UserRealtimeUpdates struct {
	app         application.Application
	userService *services.UserService
	basePath    string
}

func NewUserRealtimeUpdates(app application.Application, userService *services.UserService, basePath string) *UserRealtimeUpdates {
	return &UserRealtimeUpdates{
		app:         app,
		userService: userService,
		basePath:    basePath,
	}
}

func (ru *UserRealtimeUpdates) Register() {
	ru.app.EventPublisher().Subscribe(ru.onUserCreated)
	ru.app.EventPublisher().Subscribe(ru.onUserUpdated)
	ru.app.EventPublisher().Subscribe(ru.onUserDeleted)
}

func (ru *UserRealtimeUpdates) publisherContext() (context.Context, error) {
	localizer := i18n.NewLocalizer(ru.app.Bundle(), "en")
	ctx := composables.WithLocalizer(
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

func (ru *UserRealtimeUpdates) onUserCreated(event *user.CreatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	usr, err := ru.userService.GetByID(ctx, event.Result.ID())
	if err != nil {
		logger.Errorf("Error retrieving user: %v | Event: onUserCreated", err)
		return
	}
	component := users.UserCreatedEvent(mappers.UserToViewModel(usr), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering user row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

func (ru *UserRealtimeUpdates) onUserDeleted(event *user.DeletedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering user row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

func (ru *UserRealtimeUpdates) onUserUpdated(event *user.UpdatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	usr, err := ru.userService.GetByID(ctx, event.Result.ID())
	if err != nil {
		logger.Errorf("Error retrieving user: %v", err)
		return
	}

	component := users.UserRow(mappers.UserToViewModel(usr), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering user row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

type UsersController struct {
	app      application.Application
	basePath string
	realtime *UserRealtimeUpdates
}

func NewUsersController(app application.Application) application.Controller {
	userService := app.Service(services.UserService{}).(*services.UserService)
	basePath := "/users"

	controller := &UsersController{
		app:      app,
		basePath: basePath,
		realtime: NewUserRealtimeUpdates(app, userService, basePath),
	}

	return controller
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
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.NewHandler(c.Users).Handler()).Methods(http.MethodGet)
	router.HandleFunc("/new", di.NewHandler(c.GetNew).Handler()).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", di.NewHandler(c.GetEdit).Handler()).Methods(http.MethodGet)

	router.HandleFunc("", di.NewHandler(c.Create).Handler()).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.NewHandler(c.Update).Handler()).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.NewHandler(c.Delete).Handler()).Methods(http.MethodDelete)

	c.realtime.Register()
}

func (c *UsersController) permissionGroups(
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

	// Use the PermissionsByResource method from RBAC interface
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

func (c *UsersController) Users(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	groupService *services.GroupService,
) {
	params := composables.UsePaginated(r)
	groupIDs := r.URL.Query()["groupID"]

	// Create find params
	findParams := &user.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: user.SortBy{Fields: []user.Field{
			user.CreatedAt,
		}},
		Search: r.URL.Query().Get("Search"),
	}

	if len(groupIDs) > 0 {
		findParams.Filters = append(findParams.Filters, user.Filter{
			Column: user.GroupID,
			Filter: repo.In(groupIDs),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.Errorf("Error parsing CreatedAt.To: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, user.Filter{
			Column: user.CreatedAt,
			Filter: repo.Lt(t),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.From"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.Errorf("Error parsing CreatedAt.From: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, user.Filter{
			Column: user.CreatedAt,
			Filter: repo.Gt(t),
		})
	}

	// Get users based on filters
	us, total, err := userService.GetPaginatedWithTotal(r.Context(), findParams)
	if err != nil {
		logger.Errorf("Error retrieving users: %v", err)
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}

	// Get all groups for the sidebar
	groups, err := groupService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	props := &users.IndexPageProps{
		Users:   mapping.MapViewModels(us, mappers.UserToViewModel),
		Groups:  mapping.MapViewModels(groups, mappers.GroupToViewModel),
		Page:    params.Page,
		PerPage: params.Limit,
		HasMore: total > int64(params.Page*params.Limit),
	}

	if htmx.IsHxRequest(r) {
		if params.Page > 1 {
			templ.Handler(users.UserRows(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			if htmx.Target(r) == "users-table-body" {
				templ.Handler(users.UserRows(props), templ.WithStreaming()).ServeHTTP(w, r)
			} else {
				templ.Handler(users.UsersContent(props), templ.WithStreaming()).ServeHTTP(w, r)
			}
		}
	} else {
		templ.Handler(users.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *UsersController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupService *services.GroupService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	groups, err := groupService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	us, err := userService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving user: %v", err)
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}

	props := &users.EditFormProps{
		User:             mappers.UserToViewModel(us),
		Roles:            mapping.MapViewModels(roles, mappers.RoleToViewModel),
		Groups:           mapping.MapViewModels(groups, mappers.GroupToViewModel),
		PermissionGroups: c.permissionGroups(c.app.RBAC(), us.Permissions()...),
		Errors:           map[string]string{},
	}
	templ.Handler(users.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
	groupService *services.GroupService,
) {
	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	groups, err := groupService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	props := &users.CreateFormProps{
		User:             viewmodels.User{},
		Roles:            mapping.MapViewModels(roles, mappers.RoleToViewModel),
		Groups:           mapping.MapViewModels(groups, mappers.GroupToViewModel),
		PermissionGroups: c.permissionGroups(c.app.RBAC()),
		Errors:           map[string]string{},
	}
	templ.Handler(users.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupService *services.GroupService,
) {
	dto, err := composables.UseForm(&user.CreateDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
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

		groups, err := groupService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving groups: %v", err)
			http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		userEntity, err := dto.ToEntity()
		if err != nil {
			logger.Errorf("Error converting DTO to entity: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &users.CreateFormProps{
			User:             *mappers.UserToViewModel(userEntity),
			Roles:            mapping.MapViewModels(roles, mappers.RoleToViewModel),
			Groups:           mapping.MapViewModels(groups, mappers.GroupToViewModel),
			PermissionGroups: c.permissionGroups(c.app.RBAC()),
			Errors:           errors,
		}
		templ.Handler(
			users.CreateForm(props), templ.WithStreaming(),
		).ServeHTTP(w, r)
		return
	}

	userEntity, err := dto.ToEntity()
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the form to access form fields
	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Handle groups (if any)
	groupIDs := r.Form["GroupIDs"]
	if len(groupIDs) > 0 {
		// Convert string IDs to UUID
		ids := make([]uuid.UUID, 0, len(groupIDs))
		for _, id := range groupIDs {
			if id == "" {
				continue
			}
			if uuid, err := uuid.Parse(id); err == nil {
				ids = append(ids, uuid)
			}
		}

		if len(ids) > 0 {
			userEntity = userEntity.SetGroupIDs(ids)
		}
	}

	if err := userService.Create(r.Context(), userEntity); err != nil {
		logger.Errorf("Error creating user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *UsersController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupService *services.GroupService,
	permissionService *services.PermissionService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&user.UpdateDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
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

		groups, err := groupService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving groups: %v", err)
			http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		us, err := userService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error retrieving user: %v", err)
			http.Error(w, "Error retrieving users", http.StatusInternalServerError)
			return
		}

		props := &users.EditFormProps{
			User:             mappers.UserToViewModel(us),
			Roles:            mapping.MapViewModels(roles, mappers.RoleToViewModel),
			Groups:           mapping.MapViewModels(groups, mappers.GroupToViewModel),
			PermissionGroups: c.permissionGroups(c.app.RBAC(), us.Permissions()...),
			Errors:           errors,
		}
		templ.Handler(users.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	userEntity, err := dto.ToEntity(id)
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle permissions
	permissionIDs := r.Form["PermissionIDs"]
	permissions := []*permission.Permission{}
	for _, permID := range permissionIDs {
		if permID == "" {
			continue
		}
		perm, err := permissionService.GetByID(r.Context(), permID)
		if err != nil {
			logger.Warnf("Error retrieving permission: %v", err)
			continue
		}
		permissions = append(permissions, perm)
	}

	// Set permissions on the user entity
	userEntity = userEntity.SetPermissions(permissions)

	// Handle groups (if any)
	groupIDs := r.Form["GroupIDs"]
	if len(groupIDs) > 0 {
		// Convert string IDs to UUID
		ids := make([]uuid.UUID, 0, len(groupIDs))
		for _, id := range groupIDs {
			if id == "" {
				continue
			}
			if uuid, err := uuid.Parse(id); err == nil {
				ids = append(ids, uuid)
			}
		}

		if len(ids) > 0 {
			userEntity = userEntity.SetGroupIDs(ids)
		}
	}

	if err := userService.Update(r.Context(), userEntity); err != nil {
		logger.Errorf("Error updating user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *UsersController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := userService.Delete(r.Context(), id); err != nil {
		logger.Errorf("Error deleting user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}
