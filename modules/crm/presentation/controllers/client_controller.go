package controllers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"golang.org/x/text/language"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/base/tab"
	userdomain "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmPermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	chatsui "github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/chats"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/clients"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

func clientIDFromQ(u *url.URL) (uint, error) {
	v, err := strconv.Atoi(u.Query().Get("view"))
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

type ClientRealtimeUpdates struct {
	app           application.Application
	clientService *services.ClientService
	basePath      string
}

func NewClientRealtimeUpdates(app application.Application, clientService *services.ClientService, basePath string) *ClientRealtimeUpdates {
	return &ClientRealtimeUpdates{
		app:           app,
		clientService: clientService,
		basePath:      basePath,
	}
}

func (ru *ClientRealtimeUpdates) Register() {
	ru.app.EventPublisher().Subscribe(ru.onClientCreated)
}

func (ru *ClientRealtimeUpdates) publisherContext() (context.Context, error) {
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

func (ru *ClientRealtimeUpdates) onClientCreated(event *client.CreatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	clientEntity, err := ru.clientService.GetByID(ctx, event.Result.ID())
	if err != nil {
		logger.Errorf("Error retrieving client: %v | Event: onClientCreated", err)
		return
	}

	component := clients.ClientCreatedEvent(mappers.ClientToViewModel(clientEntity), &base.TableRowProps{
		Attrs: templ.Attributes{},
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering client row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

type TabDefinition struct {
	ID          string
	NameKey     string
	Component   func(r *http.Request, clientID uint) (templ.Component, error)
	SortOrder   int
	Permissions []*permission.Permission
}

type ClientControllerConfig struct {
	BasePath    string
	Middleware  []mux.MiddlewareFunc
	Tabs        []TabDefinition
	RealtimeBus bool
}

func DefaultClientControllerConfig() ClientControllerConfig {
	return ClientControllerConfig{
		BasePath:    "/clients",
		Middleware:  []mux.MiddlewareFunc{},
		Tabs:        []TabDefinition{}, // Empty by default, must be explicitly provided
		RealtimeBus: true,
	}
}

type ClientController struct {
	app       application.Application
	config    ClientControllerConfig
	realtime  *ClientRealtimeUpdates
	tabsByID  map[string]TabDefinition
	tabsOrder []TabDefinition
}

type ClientsPaginatedResponse struct {
	Clients         []*viewmodels.Client
	PaginationState *pagination.State
}

func NewClientController(app application.Application, config ...ClientControllerConfig) application.Controller {
	// Use default config or the provided one
	cfg := DefaultClientControllerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	clientService := app.Service(services.ClientService{}).(*services.ClientService)

	// Initialize controller
	controller := &ClientController{
		app:      app,
		config:   cfg,
		tabsByID: make(map[string]TabDefinition),
	}

	// Register provided tabs
	for _, tab := range cfg.Tabs {
		controller.RegisterTab(tab)
	}

	// Initialize realtime if enabled
	if cfg.RealtimeBus {
		controller.realtime = NewClientRealtimeUpdates(app, clientService, cfg.BasePath)
	}

	return controller
}

// Default tab definitions - exported for configuration
var (
	ProfileTab = func(basePath string) TabDefinition {
		return TabDefinition{
			ID:        "profile",
			NameKey:   "Clients.Tabs.Profile",
			SortOrder: 10,
			Permissions: []*permission.Permission{
				crmPermissions.ClientRead,
			},
			Component: func(r *http.Request, clientID uint) (templ.Component, error) {
				app, err := composables.UseApp(r.Context())
				if err != nil {
					return nil, errors.Wrap(err, "Error retrieving app")
				}
				clientService := app.Service(services.ClientService{}).(*services.ClientService)
				clientEntity, err := clientService.GetByID(r.Context(), clientID)
				if err != nil {
					return nil, errors.Wrap(err, "Error retrieving client")
				}
				return clients.Profile(clients.ProfileProps{
					ClientURL: basePath,
					EditURL:   fmt.Sprintf("%s/%d/edit", basePath, clientID),
					Client:    mappers.ClientToViewModel(clientEntity),
				}), nil
			},
		}
	}

	ChatTab = func(basePath string) TabDefinition {
		return TabDefinition{
			ID:        "chat",
			NameKey:   "Clients.Tabs.Chat",
			SortOrder: 20,
			Permissions: []*permission.Permission{
				crmPermissions.ClientRead,
			},
			Component: func(r *http.Request, clientID uint) (templ.Component, error) {
				app, err := composables.UseApp(r.Context())
				if err != nil {
					return nil, errors.Wrap(err, "Error retrieving app")
				}
				clientService := app.Service(services.ClientService{}).(*services.ClientService)
				chatService := app.Service(services.ChatService{}).(*services.ChatService)
				clientEntity, err := clientService.GetByID(r.Context(), clientID)
				if err != nil {
					return nil, errors.Wrap(err, "Error retrieving client")
				}
				chatEntity, err := chatService.GetByClientIDOrCreate(r.Context(), clientID)
				if err != nil {
					return nil, errors.Wrap(err, "Error retrieving chat")
				}
				return clients.Chats(chatsui.SelectedChatProps{
					Chat:       mappers.ChatToViewModel(chatEntity, clientEntity),
					ClientsURL: basePath,
				}), nil
			},
		}
	}

	ActionsTab = func() TabDefinition {
		return TabDefinition{
			ID:        "actions",
			NameKey:   "Clients.Tabs.Actions",
			SortOrder: 100,
			Permissions: []*permission.Permission{
				crmPermissions.ClientUpdate,
				crmPermissions.ClientDelete,
			},
			Component: func(r *http.Request, clientID uint) (templ.Component, error) {
				return clients.ActionsTab(strconv.Itoa(int(clientID))), nil
			},
		}
	}
)

func (c *ClientController) RegisterTab(tab TabDefinition) {
	c.tabsByID[tab.ID] = tab

	// Rebuild the sorted tab list
	c.tabsOrder = make([]TabDefinition, 0, len(c.tabsByID))
	for _, t := range c.tabsByID {
		c.tabsOrder = append(c.tabsOrder, t)
	}

	// Sort by SortOrder
	sort.Slice(c.tabsOrder, func(i, j int) bool {
		return c.tabsOrder[i].SortOrder < c.tabsOrder[j].SortOrder
	})
}

func (c *ClientController) Key() string {
	return c.config.BasePath
}

func (c *ClientController) Register(r *mux.Router) {
	// Combine configured middleware with required middleware
	commonMiddleware := append(
		[]mux.MiddlewareFunc{
			middleware.Authorize(),
			middleware.RedirectNotAuthenticated(),
			middleware.ProvideUser(),
			middleware.WithLocalizer(c.app.Bundle()),
			middleware.WithPageContext(),
		},
		c.config.Middleware...,
	)

	router := r.PathPrefix(c.config.BasePath).Subrouter()
	router.Use(commonMiddleware...)
	router.Use(middleware.Tabs(), middleware.NavItems())
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.Delete)).Methods(http.MethodDelete)

	hxRouter := r.PathPrefix(c.config.BasePath).Subrouter()
	hxRouter.Use(commonMiddleware...)
	hxRouter.HandleFunc("/{id:[0-9]+}", di.H(c.View)).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/personal", di.H(c.GetPersonalEdit)).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/passport", di.H(c.GetPassportEdit)).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/tax", di.H(c.GetTaxEdit)).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/personal", di.H(c.UpdatePersonal)).Methods(http.MethodPost)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/passport", di.H(c.UpdatePassport)).Methods(http.MethodPost)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/tax", di.H(c.UpdateTax)).Methods(http.MethodPost)

	// Register realtime updates if enabled
	if c.realtime != nil {
		c.realtime.Register()
	}
}

func (c *ClientController) viewModelClients(
	r *http.Request,
	clientService *services.ClientService,
) (*ClientsPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params := &client.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: client.SortBy{
			Fields:    []client.Field{client.CreatedAt},
			Ascending: false,
		},
	}

	if v := r.URL.Query().Get("CreatedAt.From"); v != "" {
		params.CreatedAt.From = v
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		params.CreatedAt.To = v
	}

	if q := r.URL.Query().Get("Query"); q != "" {
		params.Search = q
	}

	clientEntities, err := clientService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}

	total, err := clientService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &ClientsPaginatedResponse{
		Clients:         mapping.MapViewModels(clientEntities, mappers.ClientToViewModel),
		PaginationState: pagination.New(c.config.BasePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ClientController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	user userdomain.User,
	clientService *services.ClientService,
) {
	if !user.Can(crmPermissions.ClientRead) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	paginated, err := c.viewModelClients(r, clientService)
	if err != nil {
		logger.Errorf("Error retrieving clients: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := htmx.IsHxRequest(r)
	if isHxRequest && r.URL.Query().Get("view") != "" {
		c.View(r, w, user, logger, clientService, c.app.Service(services.ChatService{}).(*services.ChatService))
		return
	}
	props := &clients.IndexPageProps{
		NewURL:          fmt.Sprintf("%s/new", c.config.BasePath),
		Clients:         paginated.Clients,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(clients.ClientsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(clients.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ClientController) Create(
	r *http.Request,
	w http.ResponseWriter,
	user userdomain.User,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	if !user.Can(crmPermissions.ClientCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	dto, err := composables.UseForm(&client.CreateDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &clients.CreatePageProps{
			Errors: errorsMap,
			Client: &viewmodels.Client{
				FirstName:   dto.FirstName,
				LastName:    dto.LastName,
				MiddleName:  dto.MiddleName,
				Phone:       dto.Phone,
				Email:       dto.Email,
				Address:     dto.Address,
				Pin:         dto.Pin,
				CountryCode: dto.CountryCode,
				Passport: viewmodels.Passport{
					Series: dto.PassportSeries,
					Number: dto.PassportNumber,
				},
			},
			SaveURL: c.config.BasePath,
		}
		templ.Handler(clients.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err = clientService.Create(r.Context(), dto); err != nil {
		logger.Errorf("Error creating client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.config.BasePath)
}

func (c *ClientController) tabToComponent(
	r *http.Request,
	clientID uint,
	tabID string,
	clientService *services.ClientService,
	chatService *services.ChatService,
) (templ.Component, error) {
	// Find the tab by ID
	tab, exists := c.tabsByID[tabID]
	if !exists {
		// If the requested tab doesn't exist, return NotFound
		return clients.NotFound(), nil
	}

	// Get user from context for both permission check and passing to component
	currentUser, err := composables.UseUser(r.Context())
	if err != nil {
		// If user not found in context, redirect to NotFound
		return clients.NotFound(), nil
	}

	// Check permissions if specified
	if len(tab.Permissions) > 0 {
		// Convert permission pointers to rbac.Permission types
		perms := make([]rbac.Permission, 0, len(tab.Permissions))
		for _, p := range tab.Permissions {
			perms = append(perms, rbac.Perm(p))
		}

		// If user doesn't have any of the required permissions, return NotFound
		if !rbac.Or(perms...).Can(currentUser) {
			return clients.NotFound(), nil
		}
	}

	// Generate the component using the tab's component function
	return tab.Component(r, clientID)
}

func (c *ClientController) View(
	r *http.Request,
	w http.ResponseWriter,
	user userdomain.User,
	logger *logrus.Entry,
	clientService *services.ClientService,
	chatService *services.ChatService,
) {
	if !user.Can(crmPermissions.ClientRead) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientID, err := clientIDFromQ(r.URL)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	qTab := r.URL.Query().Get("tab")
	disablePush := r.URL.Query().Get("dp") == "true"

	// If no tab is selected, default to the first tab
	if qTab == "" && len(c.tabsOrder) > 0 {
		qTab = c.tabsOrder[0].ID
	}

	entity, err := clientService.GetByID(r.Context(), clientID)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	component, err := c.tabToComponent(r, clientID, qTab, clientService, chatService)
	if err != nil {
		logger.Errorf("Error getting tab component: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	localizer := composables.MustUseLocalizer(r.Context())

	hxCurrentURL, err := url.Parse(r.Header.Get("Hx-Current-URL"))
	if err != nil {
		logger.Errorf("Error parsing Hx-Current-URL: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := clients.ViewDrawerProps{
		SelectedTab: r.URL.RequestURI(),
		CallbackURL: hxCurrentURL.Path,
		Tabs:        []clients.ClientTab{},
	}

	// Build tabs from configured tabs
	for _, tabDef := range c.tabsOrder {
		// Check permissions if needed here
		if len(tabDef.Permissions) > 0 {
			// Convert permission pointers to rbac.Permission types
			perms := make([]rbac.Permission, 0, len(tabDef.Permissions))
			for _, p := range tabDef.Permissions {
				perms = append(perms, rbac.Perm(p))
			}

			if err := composables.CanUserAny(r.Context(), rbac.Or(perms...)); err != nil {
				continue // Skip this tab if user doesn't have permission
			}
		}

		q := url.Values{}
		q.Set("view", strconv.Itoa(int(entity.ID())))
		q.Set("tab", tabDef.ID)
		href := fmt.Sprintf("%s?%s", c.config.BasePath, q.Encode())

		props.Tabs = append(props.Tabs, clients.ClientTab{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: tabDef.NameKey,
			}),
			BoostLinkProps: tab.BoostLinkProps{
				Href: href,
				Push: !disablePush,
			},
		})
	}

	if htmx.Target(r) != "" {
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		ctx := templ.WithChildren(r.Context(), component)
		templ.Handler(clients.ViewDrawer(props), templ.WithStreaming()).ServeHTTP(w, r.WithContext(ctx))
	}
}

func (c *ClientController) GetPersonalEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, fmt.Sprintf("Error retrieving client: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	props := &clients.PersonalInfoEditProps{
		Client: mappers.ClientToViewModel(entity),
		Errors: map[string]string{},
		Form:   "personal-info-edit-form",
	}
	templ.Handler(clients.PersonalInfoEditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) GetPassportEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	props := &clients.PassportInfoEditProps{
		Client: mappers.ClientToViewModel(entity),
		Errors: map[string]string{},
		Form:   "passport-info-edit-form",
	}
	templ.Handler(clients.PassportInfoEditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) GetTaxEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	props := &clients.TaxInfoEditProps{
		Client: mappers.ClientToViewModel(entity),
		Errors: map[string]string{},
		Form:   "tax-info-edit-form",
	}
	templ.Handler(clients.TaxInfoEditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) UpdatePersonal(
	r *http.Request,
	w http.ResponseWriter,
	user userdomain.User,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	if !user.Can(crmPermissions.ClientUpdate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&client.UpdatePersonalDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := clientService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error retrieving client: %v", err)
			http.Error(w, "Error retrieving client", http.StatusInternalServerError)
			return
		}

		clientVM := mappers.ClientToViewModel(entity)
		props := &clients.PersonalInfoEditProps{
			Client: clientVM,
			Errors: errorsMap,
			Form:   "personal-info-edit-form",
		}
		templ.Handler(clients.PersonalInfoEditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	updated, err := dto.Apply(entity)
	if err != nil {
		logger.Errorf("Error applying changes: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := clientService.Save(r.Context(), updated); err != nil {
		logger.Errorf("Error saving client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err = clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving updated client: %v", err)
		http.Error(w, "Error retrieving updated client", http.StatusInternalServerError)
		return
	}

	clientVM := mappers.ClientToViewModel(entity)
	htmx.Retarget(w, "#personal-info-card")
	templ.Handler(clients.PersonalInfoCard(clientVM), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) UpdatePassport(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&client.UpdatePassportDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := clientService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error retrieving client: %v", err)
			http.Error(w, "Error retrieving client", http.StatusInternalServerError)
			return
		}

		clientVM := mappers.ClientToViewModel(entity)
		props := &clients.PassportInfoEditProps{
			Client: clientVM,
			Errors: errorsMap,
			Form:   "passport-info-edit-form",
		}
		templ.Handler(clients.PassportInfoEditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	updated, err := dto.Apply(entity)
	if err != nil {
		logger.Errorf("Error applying changes: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := clientService.Save(r.Context(), updated); err != nil {
		logger.Errorf("Error saving client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err = clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving updated client: %v", err)
		http.Error(w, "Error retrieving updated client", http.StatusInternalServerError)
		return
	}

	clientVM := mappers.ClientToViewModel(entity)
	htmx.Retarget(w, "#passport-info-card")
	templ.Handler(clients.PassportInfoCard(clientVM), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) UpdateTax(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&client.UpdateTaxDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := clientService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error retrieving client: %v", err)
			http.Error(w, "Error retrieving client", http.StatusInternalServerError)
			return
		}

		clientVM := mappers.ClientToViewModel(entity)
		props := &clients.TaxInfoEditProps{
			Client: clientVM,
			Errors: errorsMap,
			Form:   "tax-info-edit-form",
		}
		templ.Handler(clients.TaxInfoEditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving client: %v", err)
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	updated, err := dto.Apply(entity)
	if err != nil {
		logger.Errorf("Error applying changes: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := clientService.Save(r.Context(), updated); err != nil {
		logger.Errorf("Error saving client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err = clientService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving updated client: %v", err)
		http.Error(w, "Error retrieving updated client", http.StatusInternalServerError)
		return
	}

	clientVM := mappers.ClientToViewModel(entity)
	htmx.Retarget(w, "#tax-info-card")
	templ.Handler(clients.TaxInfoCard(clientVM), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	user userdomain.User,
	logger *logrus.Entry,
	clientService *services.ClientService,
) {
	if !user.Can(crmPermissions.ClientDelete) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing client ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := clientService.Delete(r.Context(), id); err != nil {
		logger.Errorf("Error deleting client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.config.BasePath)
}
