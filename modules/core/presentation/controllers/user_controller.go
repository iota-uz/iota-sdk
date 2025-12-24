package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
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
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/validators"
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

func (ru *UserRealtimeUpdates) onUserCreated(event *user.CreatedEvent) {
	logger := configuration.Use().Logger()

	component := users.UserCreatedEvent(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render user created event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send user created event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast user created event to websocket")
		return
	}
}

func (ru *UserRealtimeUpdates) onUserDeleted(event *user.DeletedEvent) {
	logger := configuration.Use().Logger()

	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{
			"hx-swap-oob": "delete",
		},
	})

	err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render user deleted event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send user deleted event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	})
	if err != nil {
		logger.WithError(err).Error("failed to broadcast user deleted event to websocket")
		return
	}
}

func (ru *UserRealtimeUpdates) onUserUpdated(event *user.UpdatedEvent) {
	logger := configuration.Use().Logger()

	component := users.UserRow(mappers.UserToViewModel(event.Result), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render user updated event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send user updated event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast user updated event to websocket")
		return
	}
}

type UsersController struct {
	app              application.Application
	basePath         string
	realtime         *UserRealtimeUpdates
	permissionSchema *rbac.PermissionSchema
}

type UsersControllerOptions struct {
	BasePath         string
	PermissionSchema *rbac.PermissionSchema
}

func NewUsersController(app application.Application, opts *UsersControllerOptions) application.Controller {
	if opts == nil || opts.PermissionSchema == nil {
		panic("UsersController requires PermissionSchema in options")
	}
	if opts.BasePath == "" {
		panic("UsersController requires explicit BasePath in options")
	}
	userService := app.Service(services.UserService{}).(*services.UserService)

	controller := &UsersController{
		app:              app,
		basePath:         opts.BasePath,
		realtime:         NewUserRealtimeUpdates(app, userService, opts.BasePath),
		permissionSchema: opts.PermissionSchema,
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
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Users)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.GetEdit)).Methods(http.MethodGet)

	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Delete)).Methods(http.MethodDelete)
	router.HandleFunc("/{id:[0-9]+}/block/drawer", di.H(c.GetBlockDrawer)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}/block", di.H(c.BlockUser)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}/unblock", di.H(c.UnblockUser)).Methods(http.MethodPost)

	c.realtime.Register()
}

func (c *UsersController) resourcePermissionGroups(
	selected ...permission.Permission,
) []*viewmodels.ResourcePermissionGroup {
	return BuildResourcePermissionGroups(c.permissionSchema, selected...)
}

// buildEditFormProps is a helper method that fetches all necessary data
// and builds the EditFormProps for rendering the edit form.
// This eliminates code duplication between GetEdit, BlockUser, and UnblockUser.
func (c *UsersController) buildEditFormProps(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
	userID uint,
) (*users.EditFormProps, error) {
	// Fetch roles
	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return nil, err
	}

	// Fetch groups
	groupParams := &query.GroupFindParams{
		Limit:   1000,
		Offset:  0,
		Filters: []query.GroupFilter{},
	}
	groups, _, err := groupQueryService.FindGroups(r.Context(), groupParams)
	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return nil, err
	}

	// Fetch user
	us, err := userService.GetByID(r.Context(), userID)
	if err != nil {
		logger.Errorf("Error retrieving user: %v", err)
		http.Error(w, "Error retrieving user", http.StatusInternalServerError)
		return nil, err
	}

	// Check if user can be deleted
	canDelete, err := userService.CanUserBeDeleted(r.Context(), userID)
	if err != nil {
		logger.Errorf("Error checking if user can be deleted: %v", err)
		http.Error(w, "Error retrieving user information", http.StatusInternalServerError)
		return nil, err
	}

	// Map user to view model
	userViewModel := mappers.UserToViewModel(us)
	userViewModel.CanDelete = canDelete

	// If user is blocked, fetch the blocker's name
	if userViewModel.IsBlocked && userViewModel.BlockedBy != "" && userViewModel.BlockedBy != "0" {
		blockedByID, parseErr := strconv.ParseUint(userViewModel.BlockedBy, 10, 64)
		if parseErr == nil {
			blockerUser, fetchErr := userService.GetByID(r.Context(), uint(blockedByID))
			if fetchErr == nil {
				blockerVM := mappers.UserToViewModel(blockerUser)
				userViewModel.BlockedByUser = blockerVM.Title()
			}
		}
	}

	// Build and return EditFormProps
	props := &users.EditFormProps{
		User:                     userViewModel,
		Roles:                    mapping.MapViewModels(roles, mappers.RoleToViewModel),
		Groups:                   groups,
		ResourcePermissionGroups: c.resourcePermissionGroups(us.Permissions()...),
		Errors:                   map[string]string{},
		CanDelete:                canDelete,
	}

	return props, nil
}

func (c *UsersController) Users(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	userQueryService *services.UserQueryService,
	groupQueryService *services.GroupQueryService,
	roleQueryService *services.RoleQueryService,
) {
	params := composables.UsePaginated(r)
	groupIDs := r.URL.Query()["groupID"]
	roleIDs := r.URL.Query()["roleID"]

	// Create find params using the query service types
	findParams := &query.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{
					Field:     "created_at",
					Ascending: false,
				},
			},
		},
		Search:  r.URL.Query().Get("Search"),
		Filters: []query.Filter{},
	}

	// Add group filter if specified
	if len(groupIDs) > 0 {
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldGroupID,
			Filter: repo.In(groupIDs),
		})
	}

	// Add role filter if specified
	if len(roleIDs) > 0 {
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldRoleID,
			Filter: repo.In(roleIDs),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.Errorf("Error parsing CreatedAt.To: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldCreatedAt,
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
		findParams.Filters = append(findParams.Filters, query.Filter{
			Column: query.FieldCreatedAt,
			Filter: repo.Gt(t),
		})
	}

	// Get users using the query service
	us, total, err := userQueryService.FindUsers(r.Context(), findParams)
	if err != nil {
		logger.Errorf("Error retrieving users: %v", err)
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}

	// Get all groups for the sidebar using query service
	groupParams := &query.GroupFindParams{
		Limit:  100, // Get a reasonable number for sidebar display
		Offset: 0,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{Field: query.GroupFieldName, Ascending: true},
			},
		},
		Filters: []query.GroupFilter{},
	}
	groups, _, err := groupQueryService.FindGroups(r.Context(), groupParams)
	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	// Get all roles with user counts for the sidebar
	roleViewModels, err := roleQueryService.GetRolesWithCounts(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	props := &users.IndexPageProps{
		Users:   us,             // Already viewmodels from query service
		Groups:  groups,         // Already viewmodels from query service
		Roles:   roleViewModels, // Mapped from domain roles
		Page:    params.Page,
		PerPage: params.Limit,
		HasMore: total > params.Page*params.Limit,
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
	groupQueryService *services.GroupQueryService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props, err := c.buildEditFormProps(r, w, logger, userService, roleService, groupQueryService, id)
	if err != nil {
		// Error already logged and response sent by buildEditFormProps
		return
	}

	templ.Handler(users.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving roles: %v", err)
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	// Use GroupQueryService to fetch all groups
	groupParams := &query.GroupFindParams{
		Limit:   1000, // Large limit to fetch all groups
		Offset:  0,
		Filters: []query.GroupFilter{},
	}
	groups, _, err := groupQueryService.FindGroups(r.Context(), groupParams)
	if err != nil {
		logger.Errorf("Error retrieving groups: %v", err)
		http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
		return
	}

	props := &users.CreateFormProps{
		User:                     viewmodels.User{},
		Roles:                    mapping.MapViewModels(roles, mappers.RoleToViewModel),
		Groups:                   groups, // Already viewmodels from query service
		ResourcePermissionGroups: c.resourcePermissionGroups(),
		Errors:                   map[string]string{},
	}
	templ.Handler(users.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	respondWithForm := func(errors map[string]string, dto *dtos.CreateUserDTO) {
		ctx := r.Context()

		roles, err := roleService.GetAll(ctx)
		if err != nil {
			logger.Errorf("Error retrieving roles: %v", err)
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}

		// Use GroupQueryService to fetch all groups
		groupParams := &query.GroupFindParams{
			Limit:   1000, // Large limit to fetch all groups
			Offset:  0,
			Filters: []query.GroupFilter{},
		}
		groups, _, err := groupQueryService.FindGroups(ctx, groupParams)
		if err != nil {
			logger.Errorf("Error retrieving groups: %v", err)
			http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		var selectedRoles []*viewmodels.Role
		for _, role := range roles {
			if slices.Contains(dto.RoleIDs, role.ID()) {
				selectedRoles = append(selectedRoles, mappers.RoleToViewModel(role))
			}
		}

		props := &users.CreateFormProps{
			User: viewmodels.User{
				FirstName:  dto.FirstName,
				LastName:   dto.LastName,
				MiddleName: dto.MiddleName,
				Email:      dto.Email,
				Phone:      dto.Phone,
				GroupIDs:   dto.GroupIDs,
				Roles:      selectedRoles,
				Language:   dto.Language,
				AvatarID:   fmt.Sprint(dto.AvatarID),
			},
			Roles:                    mapping.MapViewModels(roles, mappers.RoleToViewModel),
			Groups:                   groups, // Already viewmodels from query service
			ResourcePermissionGroups: c.resourcePermissionGroups(),
			Errors:                   errors,
		}

		templ.Handler(users.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
	}

	dto, err := composables.UseForm(&dtos.CreateUserDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errs, ok := dto.Ok(r.Context()); !ok {
		respondWithForm(errs, dto)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.Errorf("Error getting tenant: %v", err)
		http.Error(w, "Error getting tenant", http.StatusInternalServerError)
		return
	}

	userEntity, err := dto.ToEntity(tenantID)
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := userService.Create(r.Context(), userEntity); err != nil {
		var errs *validators.ValidationError
		if errors.As(err, &errs) {
			respondWithForm(errs.Fields, dto)
			return
		}

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
	groupQueryService *services.GroupQueryService,
	permissionService *services.PermissionService,
) {
	ctx := r.Context()

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

	dto, err := composables.UseForm(&dtos.UpdateUserDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondWithForm := func(errors map[string]string, dto *dtos.UpdateUserDTO) {
		us, err := userService.GetByID(ctx, id)
		if err != nil {
			logger.Errorf("Error retrieving user: %v", err)
			http.Error(w, "Error retrieving user", http.StatusInternalServerError)
			return
		}

		roles, err := roleService.GetAll(ctx)
		if err != nil {
			logger.Errorf("Error retrieving roles: %v", err)
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}

		var selectedRoles []*viewmodels.Role
		for _, role := range roles {
			if slices.Contains(dto.RoleIDs, role.ID()) {
				selectedRoles = append(selectedRoles, mappers.RoleToViewModel(role))
			}
		}

		// Use GroupQueryService to fetch all groups
		groupParams := &query.GroupFindParams{
			Limit:   1000, // Large limit to fetch all groups
			Offset:  0,
			Filters: []query.GroupFilter{},
		}
		groups, _, err := groupQueryService.FindGroups(ctx, groupParams)
		if err != nil {
			logger.Errorf("Error retrieving groups: %v", err)
			http.Error(w, "Error retrieving groups", http.StatusInternalServerError)
			return
		}

		canDelete, err := userService.CanUserBeDeleted(ctx, id)
		if err != nil {
			logger.Errorf("Error checking if user can be deleted: %v", err)
			http.Error(w, "Error retrieving user information", http.StatusInternalServerError)
			return
		}

		var avatar *viewmodels.Upload
		if us.Avatar() != nil {
			avatar = mappers.UploadToViewModel(us.Avatar())
		}

		props := &users.EditFormProps{
			User: &viewmodels.User{
				ID:          strconv.FormatUint(uint64(id), 10),
				FirstName:   dto.FirstName,
				LastName:    dto.LastName,
				MiddleName:  dto.MiddleName,
				Email:       dto.Email,
				Phone:       dto.Phone,
				Avatar:      avatar,
				Language:    dto.Language,
				LastAction:  us.LastAction().Format(time.RFC3339),
				CreatedAt:   us.CreatedAt().Format(time.RFC3339),
				Roles:       selectedRoles,
				GroupIDs:    dto.GroupIDs,
				Permissions: mapping.MapViewModels(us.Permissions(), mappers.PermissionToViewModel),
				AvatarID:    strconv.FormatUint(uint64(dto.AvatarID), 10),
				CanDelete:   canDelete,
			},
			Roles:                    mapping.MapViewModels(roles, mappers.RoleToViewModel),
			Groups:                   groups, // Already viewmodels from query service
			ResourcePermissionGroups: c.resourcePermissionGroups(us.Permissions()...),
			Errors:                   errors,
			CanDelete:                canDelete,
		}
		templ.Handler(users.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
	}

	if errs, ok := dto.Ok(ctx); !ok {
		respondWithForm(errs, dto)
		return
	}

	userEntity, err := userService.GetByID(ctx, id)
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles := make([]role.Role, 0, len(dto.RoleIDs))
	for _, rID := range dto.RoleIDs {
		r, err := roleService.GetByID(ctx, rID)
		if err != nil {
			logger.Errorf("Error getting role: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		roles = append(roles, r)
	}

	permissionIDs := r.Form["PermissionIDs"]
	permissions := make([]permission.Permission, 0, len(permissionIDs))
	for _, permID := range permissionIDs {
		if permID == "" {
			continue
		}
		perm, err := permissionService.GetByID(ctx, permID)
		if err != nil {
			logger.Warnf("Error retrieving permission: %v", err)
			continue
		}
		permissions = append(permissions, perm)
	}

	userEntity, err = dto.Apply(userEntity, roles, permissions)
	if err != nil {
		logger.Errorf("Error updating user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := userService.Update(ctx, userEntity); err != nil {
		var errs *validators.ValidationError
		if errors.As(err, &errs) {
			respondWithForm(errs.Fields, dto)
			return
		}

		logger.Errorf("Error creating user: %v", err)
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

func (c *UsersController) GetBlockDrawer(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := userService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving user: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	userVM := mappers.UserToViewModel(u)
	component := users.BlockDrawer(&users.BlockDrawerProps{
		User:   userVM,
		Errors: make(map[string]string),
	})
	if err := component.Render(r.Context(), w); err != nil {
		logger.Errorf("Error rendering block drawer: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *UsersController) BlockUser(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	pageCtx := composables.UsePageCtx(r.Context())
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	blockReason := r.FormValue("BlockReason")

	// Validate
	errors := make(map[string]string)
	if strings.TrimSpace(blockReason) == "" {
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.ReasonRequired")
	} else if len(blockReason) < 3 {
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.ReasonTooShort")
	} else if len(blockReason) > 1024 {
		errors["BlockReason"] = pageCtx.T("Users.Block.Errors.ReasonTooLong")
	}

	if len(errors) > 0 {
		u, err := userService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error fetching user: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userVM := mappers.UserToViewModel(u)
		component := users.BlockDrawer(&users.BlockDrawerProps{
			User:   userVM,
			Errors: errors,
		})
		if err := component.Render(r.Context(), w); err != nil {
			logger.Errorf("Error rendering block drawer: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// Block user
	_, err = userService.BlockUser(r.Context(), id, blockReason)
	if err != nil {
		logger.Errorf("Error blocking user: %v", err)
		errors["BlockReason"] = err.Error()
		u, err := userService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error fetching user: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userVM := mappers.UserToViewModel(u)
		component := users.BlockDrawer(&users.BlockDrawerProps{
			User:   userVM,
			Errors: errors,
		})
		if err := component.Render(r.Context(), w); err != nil {
			logger.Errorf("Error rendering block drawer: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// Build props first before any writes to avoid partial responses
	props, err := c.buildEditFormProps(r, w, logger, userService, roleService, groupQueryService, id)
	if err != nil {
		// Error already logged and response sent by buildEditFormProps
		return
	}

	// Buffer all output to write atomically
	var buf bytes.Buffer

	if _, err := fmt.Fprintf(&buf, `<div id="block-user-drawer-%d"></div>`, id); err != nil {
		logger.Errorf("Error writing block-user-drawer div: %v", err)
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}

	if _, err := buf.Write([]byte(`<div id="edit-content" hx-swap-oob="true">`)); err != nil {
		logger.Errorf("Error writing edit-content opening div: %v", err)
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}

	if err := users.EditForm(props).Render(r.Context(), &buf); err != nil {
		logger.Errorf("Error rendering edit form: %v", err)
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}

	if _, err := buf.Write([]byte(`</div>`)); err != nil {
		logger.Errorf("Error writing edit-content closing div: %v", err)
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}

	// Write all buffered content at once
	if _, err := buf.WriteTo(w); err != nil {
		logger.Errorf("Error writing response: %v", err)
		return
	}
}

func (c *UsersController) UnblockUser(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	roleService *services.RoleService,
	groupQueryService *services.GroupQueryService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing user ID: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = userService.UnblockUser(r.Context(), id)
	if err != nil {
		logger.Errorf("Error unblocking user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Success - return updated EditForm
	props, err := c.buildEditFormProps(r, w, logger, userService, roleService, groupQueryService, id)
	if err != nil {
		// Error already logged and response sent by buildEditFormProps
		return
	}

	if err := users.EditForm(props).Render(r.Context(), w); err != nil {
		logger.Errorf("Error rendering edit form: %v", err)
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
		return
	}
}
