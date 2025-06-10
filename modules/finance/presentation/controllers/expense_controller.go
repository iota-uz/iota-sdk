package controllers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
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
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
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

func (ru *ExpenseRealtimeUpdates) onExpenseCreated(event *expense.CreatedEvent) {
	logger := configuration.Use().Logger()

	component := expensesui.ExpenseRow(mappers.ExpenseToViewModel(event.Result), &templ.Attributes{})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render expense created event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send expense created event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast expense created event to websocket")
		return
	}
}

func (ru *ExpenseRealtimeUpdates) onExpenseDeleted(event *expense.DeletedEvent) {
	logger := configuration.Use().Logger()

	component := expensesui.ExpenseRow(mappers.ExpenseToViewModel(event.Result), &templ.Attributes{
		"hx-swap-oob": "delete",
	})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render expense deleted event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send expense deleted event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast expense deleted event to websocket")
		return
	}
}

func (ru *ExpenseRealtimeUpdates) onExpenseUpdated(event *expense.UpdatedEvent) {
	logger := configuration.Use().Logger()

	component := expensesui.ExpenseRow(mappers.ExpenseToViewModel(event.Result), &templ.Attributes{})

	if err := ru.app.Websocket().ForEach(application.ChannelAuthenticated, func(connCtx context.Context, conn application.Connection) error {
		var buf bytes.Buffer
		if err := component.Render(connCtx, &buf); err != nil {
			logger.WithError(err).Error("failed to render expense updated event for websocket")
			return nil // Continue processing other connections
		}
		if err := conn.SendMessage(buf.Bytes()); err != nil {
			logger.WithError(err).Error("failed to send expense updated event to websocket connection")
			return nil // Continue processing other connections
		}
		return nil
	}); err != nil {
		logger.WithError(err).Error("failed to broadcast expense updated event to websocket")
		return
	}
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
		middleware.ProvideLocalizer(c.app.Bundle()),
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
		middleware.ProvideLocalizer(c.app.Bundle()),
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
			Fields: []repo.SortByField[expense.Field]{
				{
					Field:     expense.CreatedAt,
					Ascending: false,
				},
			},
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

	dto, err := composables.UseForm(&dtos.ExpenseUpdateDTO{}, r)
	if err != nil {
		logger.Errorf("Error parsing form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	existing, err := expenseService.GetByID(r.Context(), id)
	if errors.Is(err, persistence.ErrExpenseNotFound) {
		logger.Errorf("Expense not found: %v", err)
		http.Error(w, "Expense not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Errorf("Error retrieving expense: %v", err)
		http.Error(w, "Error retrieving expense", http.StatusInternalServerError)
		return
	}

	cat, err := expenseCategoryService.GetByID(r.Context(), dto.CategoryID)
	if errors.Is(err, persistence.ErrExpenseCategoryNotFound) {
		logger.Errorf("Expense category not found: %v", err)
		http.Error(w, "Expense category not found", http.StatusBadRequest)
		return
	}

	if err != nil {
		logger.Errorf("Error retrieving expense category: %v", err)
		http.Error(w, "Error retrieving expense category", http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
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
			Expense:    mappers.ExpenseToViewModel(existing),
			Accounts:   mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
			Categories: mapping.MapViewModels(categories, mappers.ExpenseCategoryToViewModel),
			Errors:     errorsMap,
		}
		templ.Handler(expensesui.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.Apply(existing, cat)
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
