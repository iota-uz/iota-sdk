package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/clients"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ClientController struct {
	app           application.Application
	clientService *services.ClientService
	basePath      string
}

type ClientsPaginatedResponse struct {
	Clients         []*viewmodels.Client
	PaginationState *pagination.State
}

func NewClientController(app application.Application) application.Controller {
	return &ClientController{
		app:           app,
		clientService: app.Service(services.ClientService{}).(*services.ClientService),
		basePath:      "/crm/clients",
	}
}

func (c *ClientController) Key() string {
	return c.basePath
}

func (c *ClientController) viewModelClients(r *http.Request) (*ClientsPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&client.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error using query")
	}

	expenseEntities, err := c.clientService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving expenses")
	}
	viewClients := mapping.MapViewModels(expenseEntities, mappers.ClientToViewModel)

	total, err := c.clientService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &ClientsPaginatedResponse{
		Clients:         viewClients,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *ClientController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
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

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
}

func (c *ClientController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelClients(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
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

func (c *ClientController) GetNew(w http.ResponseWriter, r *http.Request) {
	props := &clients.CreatePageProps{
		Client:  &viewmodels.Client{},
		SaveURL: fmt.Sprintf("%s", c.basePath),
	}
	templ.Handler(clients.New(props), templ.WithStreaming()).ServeHTTP(w, r)
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
				FirstName:  dto.FirstName,
				LastName:   dto.LastName,
				MiddleName: dto.MiddleName,
				Phone:      dto.Phone,
			},
			SaveURL: fmt.Sprintf("%s", c.basePath),
		}
		templ.Handler(clients.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := c.clientService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)

}
