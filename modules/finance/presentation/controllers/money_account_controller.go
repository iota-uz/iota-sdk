package controllers

import (
	"fmt"
	"net/http"

	coreviewmodels "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/moneyaccounts"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type MoneyAccountController struct {
	app                 application.Application
	moneyAccountService *services.MoneyAccountService
	currencyService     *coreservices.CurrencyService
	basePath            string
}

type AccountPaginatedResponse struct {
	Accounts        []*viewmodels.MoneyAccount
	PaginationState *pagination.State
}

func NewMoneyAccountController(app application.Application) application.Controller {
	return &MoneyAccountController{
		app:                 app,
		moneyAccountService: app.Service(services.MoneyAccountService{}).(*services.MoneyAccountService),
		currencyService:     app.Service(coreservices.CurrencyService{}).(*coreservices.CurrencyService),
		basePath:            "/finance/accounts",
	}
}

func (c *MoneyAccountController) Key() string {
	return c.basePath
}

func (c *MoneyAccountController) Register(r *mux.Router) {
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
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *MoneyAccountController) viewModelCurrencies(r *http.Request) ([]*coreviewmodels.Currency, error) {
	currencies, err := c.currencyService.GetAll(r.Context())
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel), nil
}

func (c *MoneyAccountController) viewModelAccounts(r *http.Request) (*AccountPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&moneyAccount.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error using query")
	}
	accountEntities, err := c.moneyAccountService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving accounts")
	}
	viewAccounts := mapping.MapViewModels(accountEntities, mappers.MoneyAccountToViewModel)

	total, err := c.moneyAccountService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting expenses")
	}

	return &AccountPaginatedResponse{
		Accounts:        viewAccounts,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}, nil
}

func (c *MoneyAccountController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelAccounts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &moneyaccounts.IndexPageProps{
		Accounts:        paginated.Accounts,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(moneyaccounts.AccountsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(moneyaccounts.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *MoneyAccountController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	entity, err := c.moneyAccountService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving account", http.StatusInternalServerError)
		return
	}
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &moneyaccounts.EditPageProps{
		Account:    mappers.MoneyAccountToViewUpdateModel(entity),
		Currencies: currencies,
		Errors:     map[string]string{},
		PostPath:   fmt.Sprintf("%s/%s", c.basePath, id.String()),
		DeletePath: fmt.Sprintf("%s/%s", c.basePath, id.String()),
	}
	templ.Handler(moneyaccounts.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MoneyAccountController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.moneyAccountService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *MoneyAccountController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.MoneyAccountUpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.moneyAccountService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving account", http.StatusInternalServerError)
			return
		}
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &moneyaccounts.EditPageProps{
			Account:    mappers.MoneyAccountToViewUpdateModel(entity),
			Currencies: currencies,
			Errors:     errorsMap,
			PostPath:   fmt.Sprintf("%s/%s", c.basePath, id.String()),
			DeletePath: fmt.Sprintf("%s/%s", c.basePath, id.String()),
		}
		templ.Handler(moneyaccounts.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	existing, err := c.moneyAccountService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving account", http.StatusInternalServerError)
		return
	}

	entity, err := dto.Apply(existing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := c.moneyAccountService.Update(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *MoneyAccountController) GetNew(w http.ResponseWriter, r *http.Request) {
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &moneyaccounts.CreatePageProps{
		Currencies: currencies,
		Errors:     map[string]string{},
		Account:    &viewmodels.MoneyAccount{},
		PostPath:   c.basePath,
	}
	templ.Handler(moneyaccounts.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MoneyAccountController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.MoneyAccountCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tenantID, err := composables.UseTenantID(r.Context())
		if err != nil {
			http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
			return
		}

		entity, err := dto.ToEntity(tenantID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &moneyaccounts.CreatePageProps{
			Currencies: currencies,
			Errors:     errorsMap,
			Account:    mappers.MoneyAccountToViewModel(entity),
			PostPath:   c.basePath,
		}
		templ.Handler(moneyaccounts.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	entity, err := dto.ToEntity(tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.moneyAccountService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
