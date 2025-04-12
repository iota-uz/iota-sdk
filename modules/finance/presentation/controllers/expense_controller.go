package controllers

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	expensesui "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/expenses"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

type ExpenseRealtimeUpdates struct {
	app            application.Application
	expenseService *services.ExpenseService
	basePath       string
}

func NewExpenseRealtimeUpdates(app application.Application, expenseService *services.ExpenseService, basePath string) *ExpenseRealtimeUpdates {
	return &ExpenseRealtimeUpdates{
		app:            app,
		expenseService: expenseService,
		basePath:       basePath,
	}
}

func (ru *ExpenseRealtimeUpdates) Register() {
	ru.app.EventPublisher().Subscribe(ru.onExpenseCreated)
	ru.app.EventPublisher().Subscribe(ru.onExpenseUpdated)
	ru.app.EventPublisher().Subscribe(ru.onExpenseDeleted)
}

func (ru *ExpenseRealtimeUpdates) publisherContext() (context.Context, error) {
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

func (ru *ExpenseRealtimeUpdates) onExpenseCreated(event *expense.CreatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	exp, err := ru.expenseService.GetByID(ctx, event.Result.ID())
	if err != nil {
		logger.Errorf("Error retrieving expense: %v | Event: onExpenseCreated", err)
		return
	}
	component := expensesui.ExpenseRow(mappers.ExpenseToViewModel(exp), &templ.Attributes{})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering expense row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

func (ru *ExpenseRealtimeUpdates) onExpenseDeleted(event *expense.DeletedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	component := expensesui.ExpenseRow(mappers.ExpenseToViewModel(event.Result), &templ.Attributes{
		"hx-swap-oob": "delete",
	})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering expense row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

func (ru *ExpenseRealtimeUpdates) onExpenseUpdated(event *expense.UpdatedEvent) {
	logger := configuration.Use().Logger()
	ctx, err := ru.publisherContext()
	if err != nil {
		logger.Errorf("Error creating publisher context: %v", err)
		return
	}

	exp, err := ru.expenseService.GetByID(ctx, event.Result.ID())
	if err != nil {
		logger.Errorf("Error retrieving expense: %v", err)
		return
	}

	component := expensesui.ExpenseRow(mappers.ExpenseToViewModel(exp), &templ.Attributes{})

	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		logger.Errorf("Error rendering expense row: %v", err)
		return
	}

	wsHub := server.WsHub()
	wsHub.BroadcastToAll(buf.Bytes())
}

type ExpenseController struct {
	app      application.Application
	basePath string
	realtime *ExpenseRealtimeUpdates
}

type ExpensePaginationResponse struct {
	Expenses        []*viewmodels.Expense
	PaginationState *pagination.State
}

func NewExpensesController(app application.Application) application.Controller {
	expenseService := app.Service(services.ExpenseService{}).(*services.ExpenseService)
	basePath := "/finance/expenses"

	controller := &ExpenseController{
		app:      app,
		basePath: basePath,
		realtime: NewExpenseRealtimeUpdates(app, expenseService, basePath),
	}

	return controller
}

func (c *ExpenseController) Key() string {
	return c.basePath
}

func (c *ExpenseController) Register(r *mux.Router) {
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
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", di.H(c.GetEdit)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
		middleware.WithTransaction(),
	)
	setRouter.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", di.H(c.Update)).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", di.H(c.Delete)).Methods(http.MethodDelete)

	c.realtime.Register()
}

func (c *ExpenseController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
) {
	params := composables.UsePaginated(r)
	findParams := &expense.FindParams{
		Offset: params.Offset,
		Limit:  params.Limit,
		SortBy: expense.SortBy{
			Fields: []expense.Field{
				expense.CreatedAt,
			},
			Ascending: false,
		},
		Search: r.URL.Query().Get("Search"),
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.Errorf("Error parsing CreatedAt.To: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		findParams.Filters = append(findParams.Filters, expense.Filter{
			Column: expense.CreatedAt,
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
		findParams.Filters = append(findParams.Filters, expense.Filter{
			Column: expense.CreatedAt,
			Filter: repo.Gt(t),
		})
	}

	expenseEntities, err := expenseService.GetPaginated(r.Context(), findParams)
	if err != nil {
		logger.Errorf("Error retrieving expenses: %v", err)
		http.Error(w, "Error retrieving expenses", http.StatusInternalServerError)
		return
	}

	total, err := expenseService.Count(r.Context(), &expense.FindParams{})
	if err != nil {
		logger.Errorf("Error counting expenses: %v", err)
		http.Error(w, "Error counting expenses", http.StatusInternalServerError)
		return
	}

	props := &expensesui.IndexPageProps{
		Expenses:        mapping.MapViewModels(expenseEntities, mappers.ExpenseToViewModel),
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(expensesui.ExpensesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(expensesui.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ExpenseController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing expense ID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := expenseService.GetByID(r.Context(), id)
	if err != nil {
		logger.Errorf("Error retrieving expense: %v", err)
		http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
		return
	}

	accounts, err := moneyAccountService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving accounts: %v", err)
		http.Error(w, "Error retrieving accounts", http.StatusInternalServerError)
		return
	}

	categories, err := expenseCategoryService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving categories: %v", err)
		http.Error(w, "Error retrieving categories", http.StatusInternalServerError)
		return
	}

	props := &expensesui.EditPageProps{
		Expense:    mappers.ExpenseToViewModel(entity),
		Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
		Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
		Errors:     map[string]string{},
	}
	templ.Handler(expensesui.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing expense ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := expenseService.Delete(r.Context(), id); err != nil {
		logger.Errorf("Error deleting expense: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		logger.Errorf("Error parsing expense ID: %v", err)
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	dto := dtos.ExpenseUpdateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		logger.Errorf("Error decoding form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := expenseService.GetByID(r.Context(), id)
		if err != nil {
			logger.Errorf("Error retrieving expense: %v", err)
			http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
			return
		}

		accounts, err := moneyAccountService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving accounts: %v", err)
			http.Error(w, "Error retrieving accounts", http.StatusInternalServerError)
			return
		}

		categories, err := expenseCategoryService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving categories: %v", err)
			http.Error(w, "Error retrieving categories", http.StatusInternalServerError)
			return
		}

		props := &expensesui.EditPageProps{
			Expense:    mappers.ExpenseToViewModel(entity),
			Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
			Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
			Errors:     errorsMap,
		}
		templ.Handler(expensesui.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity(id)
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := expenseService.Update(r.Context(), entity); err != nil {
		logger.Errorf("Error updating expense: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ExpenseController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	accounts, err := moneyAccountService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving accounts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categories, err := expenseCategoryService.GetAll(r.Context())
	if err != nil {
		logger.Errorf("Error retrieving categories: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &expensesui.CreatePageProps{
		Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
		Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
		Errors:     map[string]string{},
		Expense: mappers.ExpenseToViewModel(expense.New(
			0,
			moneyaccount.Account{},
			category.New(
				"",            // name
				0.0,           // amount - using 0.0 to be explicit about float64
				&currency.USD, // currency
			),
			time.Now(),
		)),
	}
	templ.Handler(expensesui.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ExpenseController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	expenseService *services.ExpenseService,
	moneyAccountService *services.MoneyAccountService,
	expenseCategoryService *services.ExpenseCategoryService,
) {
	if err := r.ParseForm(); err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := dtos.ExpenseCreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		logger.Errorf("Error decoding form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		accounts, err := moneyAccountService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving accounts: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		entity, err := dto.ToEntity()
		if err != nil {
			logger.Errorf("Error converting DTO to entity: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		categories, err := expenseCategoryService.GetAll(r.Context())
		if err != nil {
			logger.Errorf("Error retrieving categories: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		props := &expensesui.CreatePageProps{
			Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
			Errors:     errorsMap,
			Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
			Expense:    mappers.ExpenseToViewModel(entity),
		}
		templ.Handler(expensesui.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		logger.Errorf("Error converting DTO to entity: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := expenseService.Create(r.Context(), entity); err != nil {
		logger.Errorf("Error creating expense: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
