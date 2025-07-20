package controllers

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/components"
	transactions "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/transactions"

	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type TransactionController struct {
	app                application.Application
	transactionService *services.TransactionService
	queryRepo          query.TransactionQueryRepository
	basePath           string
	tableDefinition    table.TableDefinition
}

func NewTransactionController(app application.Application) application.Controller {
	basePath := "/finance/transactions"

	// Create table definition once at initialization
	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &TransactionController{
		app:                app,
		transactionService: app.Service(services.TransactionService{}).(*services.TransactionService),
		queryRepo:          query.NewPgTransactionQueryRepository(),
		basePath:           basePath,
		tableDefinition:    tableDefinition,
	}
}

func (c *TransactionController) Key() string {
	return c.basePath
}

func (c *TransactionController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(commonMiddleware...)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", c.GetViewDrawer).Methods(http.MethodGet)
}

func (c *TransactionController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	paginationParams := composables.UsePaginated(r)

	// Build query params
	params := &query.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{
					Field:     query.FieldTransactionDate,
					Ascending: false,
				},
			},
		},
		Search: r.URL.Query().Get("search"),
	}

	// Add filters
	var filters []query.Filter
	if transactionType := r.URL.Query().Get("type"); transactionType != "" {
		filters = append(filters, query.Filter{
			Column: query.FieldTransactionType,
			Filter: repo.Eq(transactionType),
		})
	}
	if accountID := r.URL.Query().Get("account"); accountID != "" {
		filters = append(filters, query.Filter{
			Column: query.FieldOriginAccountID,
			Filter: repo.Eq(accountID),
		})
	}

	// Add transaction date filter
	if dateFrom := r.URL.Query().Get("TransactionDate.From"); dateFrom != "" {
		if fromTime, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			filters = append(filters, query.Filter{
				Column: query.FieldTransactionDate,
				Filter: repo.Gte(fromTime),
			})
		}
	}
	if dateTo := r.URL.Query().Get("TransactionDate.To"); dateTo != "" {
		if toTime, err := time.Parse(time.RFC3339, dateTo); err == nil {
			filters = append(filters, query.Filter{
				Column: query.FieldTransactionDate,
				Filter: repo.Lte(toTime),
			})
		}
	}

	// Add accounting period filter
	if periodFrom := r.URL.Query().Get("AccountingPeriod.From"); periodFrom != "" {
		if fromTime, err := time.Parse(time.RFC3339, periodFrom); err == nil {
			filters = append(filters, query.Filter{
				Column: query.FieldAccountingPeriod,
				Filter: repo.Gte(fromTime),
			})
		}
	}
	if periodTo := r.URL.Query().Get("AccountingPeriod.To"); periodTo != "" {
		if toTime, err := time.Parse(time.RFC3339, periodTo); err == nil {
			filters = append(filters, query.Filter{
				Column: query.FieldAccountingPeriod,
				Filter: repo.Lte(toTime),
			})
		}
	}

	params.Filters = filters

	// Get transactions with populated data
	transactionVMs, total, err := c.queryRepo.FindTransactions(ctx, params)
	if err != nil {
		http.Error(w, "Error retrieving transactions", http.StatusInternalServerError)
		return
	}

	// Create table definition with localized values (only for full page render)
	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		definition = table.NewTableDefinition(
			pageCtx.T("Transactions.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("date", pageCtx.T("Transactions.List.Date")),
				table.Column("type", pageCtx.T("Transactions.List.Type")),
				table.Column("amount", pageCtx.T("Transactions.List.Amount")),
				table.Column("account", pageCtx.T("Transactions.List.Account")),
				table.Column("category", pageCtx.T("Transactions.List.Category")),
				table.Column("counterparty", pageCtx.T("Transactions.List.Counterparty")),
				table.Column("comment", pageCtx.T("Transactions.List.Comment")),
			).
			WithFilters(
				components.TransactionTypeFilter(),
				components.TransactionDateFilter(),
				components.AccountingPeriodFilter(),
			).
			WithInfiniteScroll(true).
			Build()
	} else {
		// For HTMX requests, use minimal definition
		definition = c.tableDefinition
	}

	// Build table rows
	rows := make([]table.TableRow, 0, len(transactionVMs))

	for _, tx := range transactionVMs {
		listItem := mappers.TransactionToListItem(tx)

		// Account name
		accountName := ""
		if listItem.Account != nil {
			accountName = listItem.Account.Name
		}

		// Category name
		categoryName := ""
		if listItem.Category != nil {
			categoryName = listItem.Category.Name
		}

		// Counterparty name
		counterpartyName := ""
		if listItem.Counterparty != nil {
			counterpartyName = listItem.Counterparty.Name
		}

		cells := []templ.Component{
			table.DateTime(listItem.TransactionDate),
			components.TransactionTypeBadge(listItem.TransactionType),
			templ.Raw(listItem.AmountWithCurrency),
			templ.Raw(accountName),
			templ.Raw(categoryName),
			templ.Raw(counterpartyName),
			templ.Raw(listItem.Comment),
		}

		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(c.basePath + "/" + listItem.ID + "/drawer"),
		)
		rows = append(rows, row)
	}

	// Create table data
	tableData := table.NewTableData().
		WithRows(rows...).
		WithPagination(paginationParams.Page, paginationParams.Limit, int64(total)).
		WithQueryParams(r.URL.Query())

	// Create renderer and render appropriate component
	renderer := table.NewTableRenderer(definition, tableData)

	if htmx.IsHxRequest(r) {
		templ.Handler(renderer.RenderRows(), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(renderer.RenderFull(), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *TransactionController) GetViewDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transaction, err := c.queryRepo.FindTransactionByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving transaction", http.StatusInternalServerError)
		return
	}

	props := &transactions.DrawerViewProps{
		Transaction: transaction,
	}
	templ.Handler(transactions.ViewDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}
