package controllers

import (
	"fmt"
	coreservices "github.com/iota-agency/iota-sdk/modules/core/services"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"net/http"

	"github.com/go-faster/errors"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	moneyAccount "github.com/iota-agency/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-agency/iota-sdk/modules/finance/services"
	"github.com/iota-agency/iota-sdk/modules/finance/templates/pages/moneyaccounts"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	coremappers "github.com/iota-agency/iota-sdk/pkg/presentation/mappers"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules/finance/mappers"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/presentation/viewmodels"
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
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
	}

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *MoneyAccountController) viewModelCurrencies(r *http.Request) ([]*viewmodels.Currency, error) {
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
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Accounts.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paginated, err := c.viewModelAccounts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &moneyaccounts.IndexPageProps{
		PageContext:     pageCtx,
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
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Accounts.Meta.Edit.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		PageContext: pageCtx,
		Account:     mappers.MoneyAccountToViewUpdateModel(entity),
		Currencies:  currencies,
		Errors:      map[string]string{},
		PostPath:    fmt.Sprintf("%s/%d", c.basePath, id),
		DeletePath:  fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(moneyaccounts.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MoneyAccountController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
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

func (c *MoneyAccountController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	action := shared.FormAction(r.FormValue("_action"))
	if !action.IsValid() {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	r.Form.Del("_action")

	switch action {
	case shared.FormActionDelete:
		if _, err := c.moneyAccountService.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case shared.FormActionSave:
		dto := moneyAccount.UpdateDTO{} //nolint:exhaustruct
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, types.NewPageData("Accounts.Meta.Edit.Title", ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		errorsMap, ok := dto.Ok(pageCtx.UniTranslator)
		if ok {
			if err := c.moneyAccountService.Update(r.Context(), id, &dto); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
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
				PageContext: pageCtx,
				Account:     mappers.MoneyAccountToViewUpdateModel(entity),
				Currencies:  currencies,
				Errors:      errorsMap,
				PostPath:    fmt.Sprintf("%s/%d", c.basePath, id),
				DeletePath:  fmt.Sprintf("%s/%d", c.basePath, id),
			}
			templ.Handler(moneyaccounts.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *MoneyAccountController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Accounts.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &moneyaccounts.CreatePageProps{
		PageContext: pageCtx,
		Currencies:  currencies,
		Errors:      map[string]string{},
		Account:     mappers.MoneyAccountToViewModel(&moneyAccount.Account{}), //nolint:exhaustruct
		PostPath:    c.basePath,
	}
	templ.Handler(moneyaccounts.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MoneyAccountController) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := moneyAccount.CreateDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Accounts.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &moneyaccounts.CreatePageProps{
			PageContext: pageCtx,
			Currencies:  currencies,
			Errors:      errorsMap,
			Account:     mappers.MoneyAccountToViewModel(entity),
			PostPath:    c.basePath,
		}
		templ.Handler(moneyaccounts.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.moneyAccountService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
