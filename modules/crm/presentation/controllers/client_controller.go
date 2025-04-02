package controllers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"golang.org/x/text/language"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/base/tab"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	chatsui "github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/chats"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/clients"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
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

type ClientController struct {
	app           application.Application
	clientService *services.ClientService
	chatService   *services.ChatService
	basePath      string
	realtime      *ClientRealtimeUpdates
}

type ClientsPaginatedResponse struct {
	Clients         []*viewmodels.Client
	PaginationState *pagination.State
}

func NewClientController(app application.Application, basePath string) application.Controller {
	return &ClientController{
		app:           app,
		clientService: app.Service(services.ClientService{}).(*services.ClientService),
		chatService:   app.Service(services.ChatService{}).(*services.ChatService),
		basePath:      basePath,
		realtime:      NewClientRealtimeUpdates(app, app.Service(services.ClientService{}).(*services.ClientService), basePath),
	}
}

func (c *ClientController) Key() string {
	return c.basePath
}

func (c *ClientController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.WithPageContext(),
	}
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(commonMiddleware...)
	router.Use(middleware.Tabs(), middleware.NavItems())
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)

	hxRouter := r.PathPrefix(c.basePath).Subrouter()
	hxRouter.Use(commonMiddleware...)
	hxRouter.HandleFunc("/{id:[0-9]+}", c.View).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/personal", c.GetPersonalEdit).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/passport", c.GetPassportEdit).Methods(http.MethodGet)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/tax", c.GetTaxEdit).Methods(http.MethodGet)

	hxRouter.HandleFunc("/{id:[0-9]+}/edit/personal", c.UpdatePersonal).Methods(http.MethodPost)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/passport", c.UpdatePassport).Methods(http.MethodPost)
	hxRouter.HandleFunc("/{id:[0-9]+}/edit/tax", c.UpdateTax).Methods(http.MethodPost)

	c.realtime.Register()
}

func (c *ClientController) viewModelClients(r *http.Request) (*ClientsPaginatedResponse, error) {
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

	clientEntities, err := c.clientService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}

	total, err := c.clientService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &ClientsPaginatedResponse{
		Clients:         mapping.MapViewModels(clientEntities, mappers.ClientToViewModel),
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ClientController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelClients(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := htmx.IsHxRequest(r)
	if isHxRequest && r.URL.Query().Get("view") != "" {
		c.View(w, r)
		return
	}
	props := &clients.IndexPageProps{
		NewURL:          fmt.Sprintf("%s/new", c.basePath),
		Clients:         paginated.Clients,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(clients.ClientsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(clients.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ClientController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&client.CreateDTO{}, r)
	if err != nil {
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
			SaveURL: c.basePath,
		}
		templ.Handler(clients.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err = c.clientService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)

}

func (c *ClientController) tabToComponent(
	r *http.Request,
	clientID uint,
	tab string,
) (templ.Component, error) {
	clientEntity, err := c.clientService.GetByID(r.Context(), clientID)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving client")
	}

	switch tab {
	case "profile":
		return clients.Profile(clients.ProfileProps{
			ClientURL: c.basePath,
			EditURL:   fmt.Sprintf("%s/%d/edit", c.basePath, clientID),
			Client:    mappers.ClientToViewModel(clientEntity),
		}), nil
	case "chat":
		chatEntity, err := c.chatService.GetByClientIDOrCreate(r.Context(), clientID)
		if err != nil {
			return nil, errors.Wrap(err, "Error retrieving chat")
		}
		return clients.Chats(chatsui.SelectedChatProps{
			Chat:       mappers.ChatToViewModel(chatEntity, clientEntity),
			ClientsURL: c.basePath,
		}), nil
	default:
		return clients.NotFound(), nil
	}
}

func (c *ClientController) View(w http.ResponseWriter, r *http.Request) {
	clientID, err := clientIDFromQ(r.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	qTab := r.URL.Query().Get("tab")
	disablePush := r.URL.Query().Get("dp") == "true"

	entity, err := c.clientService.GetByID(r.Context(), clientID)
	if err != nil {
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}
	component, err := c.tabToComponent(r, clientID, qTab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	localizer := composables.MustUseLocalizer(r.Context())

	hxCurrentURL, err := url.Parse(r.Header.Get("Hx-Current-URL"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := clients.ViewDrawerProps{
		SelectedTab: r.URL.RequestURI(),
		CallbackURL: hxCurrentURL.Path,
		Tabs:        []clients.ClientTab{},
	}
	tabs := []struct {
		Name  string
		Value string
	}{
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "Clients.Tabs.Profile",
			}),
			Value: "profile",
		},
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "Clients.Tabs.Chat",
			}),
			Value: "chat",
		},
	}
	for _, t := range tabs {
		q := url.Values{}
		q.Set("view", strconv.Itoa(int(entity.ID())))
		q.Set("tab", t.Value)
		href := fmt.Sprintf("%s?%s", c.basePath, q.Encode())
		props.Tabs = append(props.Tabs, clients.ClientTab{
			Name: t.Name,
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

func (c *ClientController) GetPersonalEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.clientService.GetByID(r.Context(), id)
	if err != nil {
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

func (c *ClientController) GetPassportEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.clientService.GetByID(r.Context(), id)
	if err != nil {
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

func (c *ClientController) GetTaxEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.clientService.GetByID(r.Context(), id)
	if err != nil {
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

func (c *ClientController) UpdatePersonal(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&client.UpdatePersonalDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.clientService.GetByID(r.Context(), id)
		if err != nil {
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

	entity, err := c.clientService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	updated, err := dto.Apply(entity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := c.clientService.Save(r.Context(), updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err = c.clientService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving updated client", http.StatusInternalServerError)
		return
	}

	clientVM := mappers.ClientToViewModel(entity)
	htmx.Retarget(w, "#personal-info-card")
	templ.Handler(clients.PersonalInfoCard(clientVM), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) UpdatePassport(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&client.UpdatePassportDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.clientService.GetByID(r.Context(), id)
		if err != nil {
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

	entity, err := c.clientService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	updated, err := dto.Apply(entity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := c.clientService.Save(r.Context(), updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err = c.clientService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving updated client", http.StatusInternalServerError)
		return
	}

	clientVM := mappers.ClientToViewModel(entity)
	htmx.Retarget(w, "#passport-info-card")
	templ.Handler(clients.PassportInfoCard(clientVM), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) UpdateTax(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&client.UpdateTaxDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.clientService.GetByID(r.Context(), id)
		if err != nil {
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

	entity, err := c.clientService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving client", http.StatusInternalServerError)
		return
	}

	updated, err := dto.Apply(entity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := c.clientService.Save(r.Context(), updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err = c.clientService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving updated client", http.StatusInternalServerError)
		return
	}

	clientVM := mappers.ClientToViewModel(entity)
	htmx.Retarget(w, "#tax-info-card")
	templ.Handler(clients.TaxInfoCard(clientVM), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ClientController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.clientService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}
