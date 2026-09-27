// Package controllers provides this package.
package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	coreviewmodels "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/moneyaccounts"

	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type MoneyAccountController struct {
	moneyAccountService *services.MoneyAccountService
	transactionService  *services.TransactionService
	currencyService     *coreservices.CurrencyService
	transactionQuery    query.TransactionQueryRepository
	basePath            string
	tableDefinition     table.TableDefinition
}

func NewMoneyAccountController(
	moneyAccountService *services.MoneyAccountService,
	transactionService *services.TransactionService,
	currencyService *coreservices.CurrencyService,
) application.Controller {
	basePath := "/finance/accounts"

	// Create table definition with columns for HTMX requests
	tableDefinition := table.NewTableDefinition("", basePath).
		WithColumns(
			table.Column("name", "Name"),
			table.Column("balance", "Balance"),
			table.Column("account_number", "Account Number"),
			table.Column("created_at", "Created At"),
		).
		WithInfiniteScroll(true).
		Build()

	return &MoneyAccountController{
		moneyAccountService: moneyAccountService,
		transactionService:  transactionService,
		currencyService:     currencyService,
		transactionQuery:    query.NewPgTransactionQueryRepository(),
		basePath:            basePath,
		tableDefinition:     tableDefinition,
	}
}

func (c *MoneyAccountController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("finance.money_account", 0, application.Route("", c.basePath)).
		WithNav(application.NavNode{
			ID:       "finance.money_account",
			Parent:   "finance",
			TitleKey: "NavigationLinks.Accounts",
			Path:     c.basePath,
			Order:    40,
			Actions: []application.NavAction{{
				ID:       "finance.money_account.new",
				TitleKey: "MoneyAccounts.List.New",
				Path:     c.basePath + "/new",
			}},
		})
}

func (c *MoneyAccountController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(commonMiddleware...)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", c.GetEditDrawer).Methods(http.MethodGet)
	router.HandleFunc("/new/drawer", c.GetNewDrawer).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/transfer/drawer", c.GetTransferDrawer).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/transfer", c.CreateTransfer).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/transactions", c.GetAccountTransactions).Methods(http.MethodGet)
}

func (c *MoneyAccountController) viewModelCurrencies(r *http.Request) ([]*coreviewmodels.Currency, error) {
	currencies, err := c.currencyService.GetAll(r.Context())
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel), nil
}

func (c *MoneyAccountController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	paginationParams := composables.UsePaginated(r)
	params := &moneyAccount.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: moneyAccount.SortBy{
			Fields: []repo.SortByField[moneyAccount.Field]{
				{
					Field:     moneyAccount.CreatedAt,
					Ascending: false,
				},
			},
		},
	}

	if search := table.UseSearchQuery(r); search != "" {
		params.Search = search
	}

	if from := r.URL.Query().Get("CreatedAt.From"); from != "" {
		if to := r.URL.Query().Get("CreatedAt.To"); to != "" {
			fromTime, err := time.Parse(time.RFC3339, from)
			if err != nil {
				http.Error(w, "Invalid from date format", http.StatusBadRequest)
				return
			}
			toTime, err := time.Parse(time.RFC3339, to)
			if err != nil {
				http.Error(w, "Invalid to date format", http.StatusBadRequest)
				return
			}
			params.Filters = append(params.Filters, moneyAccount.Filter{
				Column: moneyAccount.CreatedAt,
				Filter: repo.Between(fromTime, toTime),
			})
		}
	}

	accountEntities, err := c.moneyAccountService.GetPaginated(ctx, params)
	if err != nil {
		http.Error(w, "Error retrieving money accounts", http.StatusInternalServerError)
		return
	}

	total, err := c.moneyAccountService.Count(ctx, params)
	if err != nil {
		http.Error(w, "Error counting money accounts", http.StatusInternalServerError)
		return
	}

	// Create table definition with localized values (only for full page render)
	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		// Create action for drawer
		createAction := actions.CreateAction(
			pageCtx.T("MoneyAccounts.List.New"),
			"",
		)
		createAction.Attrs = templ.Attributes{
			"hx-get":    c.basePath + "/new/drawer",
			"hx-target": "#view-drawer",
			"hx-swap":   "innerHTML",
		}

		definition = table.NewTableDefinition(
			pageCtx.T("MoneyAccounts.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("name", pageCtx.T("MoneyAccounts.List.Name")),
				table.Column("balance", pageCtx.T("MoneyAccounts.List.Balance")),
				table.Column("account_number", pageCtx.T("MoneyAccounts.Single.AccountNumber")),
				table.Column("created_at", pageCtx.T("CreatedAt")),
			).
			WithActions(actions.RenderAction(createAction)).
			WithFilters(filters.CreatedAt()).
			WithDeferredPanels(table.DeferredPanel{ID: "account-balances", URL: "/finance/balances"}).
			WithInfiniteScroll(true).
			Build()
	} else {
		// For HTMX requests, use minimal definition
		definition = c.tableDefinition
	}

	// Build table rows
	viewAccounts := mapping.MapViewModels(accountEntities, mappers.MoneyAccountToViewModel)
	rows := make([]table.TableRow, 0, len(viewAccounts))

	for _, account := range viewAccounts {
		createdAt, err := time.Parse("2006-01-02T15:04:05Z07:00", account.CreatedAt)
		if err != nil {
			createdAt, err = time.Parse("2006-01-02 15:04:05", account.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}
		}

		cells := []table.TableCell{
			table.Cell(templ.Raw(account.Name), account.Name),
			table.Cell(templ.Raw(account.BalanceWithCurrency), account.BalanceWithCurrency),
			table.Cell(templ.Raw(account.AccountNumber), account.AccountNumber),
			table.Cell(table.DateTime(createdAt), createdAt),
		}

		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(fmt.Sprintf("%s/%s/drawer", c.basePath, account.ID)),
		)
		rows = append(rows, row)
	}

	// Create table data
	tableData := table.NewTableData().
		WithRows(rows...).
		WithPagination(paginationParams.Page, paginationParams.Limit, total).
		WithQueryParams(r.URL.Query())

	// Create renderer and render appropriate component
	renderer := table.NewTableRenderer(definition, tableData)

	if htmx.IsHxRequest(r) {
		templ.Handler(renderer.RenderRows(), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(renderer.RenderFull(), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *MoneyAccountController) GetEditDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.moneyAccountService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving money account", http.StatusInternalServerError)
		return
	}

	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	history, err := c.accountHistory(r, id)
	if err != nil {
		http.Error(w, "Error retrieving account transactions", http.StatusInternalServerError)
		return
	}

	props := &moneyaccounts.DrawerEditProps{
		Account:    mappers.MoneyAccountToViewModel(entity),
		UpdateData: mappers.MoneyAccountToViewUpdateModel(entity),
		Currencies: currencies,
		History:    history,
		Errors:     map[string]string{},
	}
	templ.Handler(moneyaccounts.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MoneyAccountController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &moneyaccounts.DrawerCreateProps{
		Errors:     map[string]string{},
		Account:    dtos.MoneyAccountCreateDTO{},
		Currencies: currencies,
	}
	templ.Handler(moneyaccounts.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
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

	isDrawer := htmx.Target(r) != "" && htmx.Target(r) != "edit-content"

	if errorsMap, ok := dto.Ok(r.Context()); ok {
		existing, err := c.moneyAccountService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving money account", http.StatusInternalServerError)
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

		// Always redirect to refresh the table
		shared.Redirect(w, r, c.basePath)
	} else {
		entity, err := c.moneyAccountService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving money account", http.StatusInternalServerError)
			return
		}

		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if isDrawer {
			history, err := c.accountHistory(r, id)
			if err != nil {
				http.Error(w, "Error retrieving account transactions", http.StatusInternalServerError)
				return
			}
			props := &moneyaccounts.DrawerEditProps{
				Account:    mappers.MoneyAccountToViewModel(entity),
				UpdateData: dto.ToViewModel(id),
				Currencies: currencies,
				History:    history,
				Errors:     errorsMap,
			}
			templ.Handler(moneyaccounts.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Edit form not supported - use drawer", http.StatusBadRequest)
		}
	}
}

func (c *MoneyAccountController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.MoneyAccountCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.IsHxRequest(r) && htmx.Target(r) == "money-account-create-drawer"

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if isDrawer {
			props := &moneyaccounts.DrawerCreateProps{
				Errors:     errorsMap,
				Account:    *dto,
				Currencies: currencies,
			}
			templ.Handler(moneyaccounts.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Create form not supported - use drawer", http.StatusBadRequest)
		}
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

func (c *MoneyAccountController) GetTransferDrawer(w http.ResponseWriter, r *http.Request) {
	sourceAccountID, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sourceAccount, err := c.moneyAccountService.GetByID(r.Context(), sourceAccountID)
	if err != nil {
		http.Error(w, "Error retrieving source account", http.StatusInternalServerError)
		return
	}

	// Get all other accounts for destination selection (excluding source account)
	allAccounts, err := c.moneyAccountService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving accounts", http.StatusInternalServerError)
		return
	}

	// Filter out the source account
	var destinationAccounts []moneyAccount.Account
	for _, account := range allAccounts {
		if account.ID() != sourceAccountID {
			destinationAccounts = append(destinationAccounts, account)
		}
	}

	props := &moneyaccounts.TransferDrawerProps{
		SourceAccount:       mappers.MoneyAccountToViewModel(sourceAccount),
		DestinationAccounts: mapping.MapViewModels(destinationAccounts, mappers.MoneyAccountToViewModel),
		TransferData:        dtos.TransferDTO{},
		Errors:              map[string]string{},
	}
	templ.Handler(moneyaccounts.TransferDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *MoneyAccountController) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	sourceAccountID, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&dtos.TransferDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		// Return form with errors
		sourceAccount, err := c.moneyAccountService.GetByID(r.Context(), sourceAccountID)
		if err != nil {
			http.Error(w, "Error retrieving source account", http.StatusInternalServerError)
			return
		}

		allAccounts, err := c.moneyAccountService.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Error retrieving accounts", http.StatusInternalServerError)
			return
		}

		var destinationAccounts []moneyAccount.Account
		for _, account := range allAccounts {
			if account.ID() != sourceAccountID {
				destinationAccounts = append(destinationAccounts, account)
			}
		}

		props := &moneyaccounts.TransferDrawerProps{
			SourceAccount:       mappers.MoneyAccountToViewModel(sourceAccount),
			DestinationAccounts: mapping.MapViewModels(destinationAccounts, mappers.MoneyAccountToViewModel),
			TransferData:        *dto,
			Errors:              errorsMap,
		}
		templ.Handler(moneyaccounts.TransferDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Get tenant ID
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	// Get source account to determine currency
	sourceAccount, err := c.moneyAccountService.GetByID(r.Context(), sourceAccountID)
	if err != nil {
		http.Error(w, "Error retrieving source account", http.StatusInternalServerError)
		return
	}

	// Parse destination account ID
	destinationAccountID, err := dto.GetDestinationAccountID()
	if err != nil {
		http.Error(w, "Invalid destination account ID", http.StatusBadRequest)
		return
	}

	// Get destination account to check currency compatibility
	destinationAccount, err := c.moneyAccountService.GetByID(r.Context(), destinationAccountID)
	if err != nil {
		http.Error(w, "Error retrieving destination account", http.StatusInternalServerError)
		return
	}

	// Check if this is a cross-currency transfer
	sourceCurrency := sourceAccount.Balance().Currency().Code
	destCurrency := destinationAccount.Balance().Currency().Code
	isCrossCurrency := sourceCurrency != destCurrency

	// Create transaction amount using source account's currency
	transferAmount := money.NewFromFloat(dto.Amount, sourceCurrency)

	now := time.Now()
	var transferTransaction transaction.Transaction

	if isCrossCurrency || dto.IsExchange {
		// Cross-currency transfer - create an EXCHANGE transaction
		if !dto.IsExchange || dto.ExchangeRate == nil || *dto.ExchangeRate <= 0 {
			http.Error(w, "Exchange rate is required for cross-currency transfers", http.StatusBadRequest)
			return
		}

		// Create destination amount in destination currency
		destinationAmount := money.NewFromFloat(dto.GetDestinationAmount(), destCurrency)

		transferTransaction = transaction.New(
			transferAmount,
			transaction.Exchange,
			transaction.WithTenantID(tenantID),
			transaction.WithOriginAccountID(sourceAccountID),
			transaction.WithDestinationAccountID(destinationAccountID),
			transaction.WithTransactionDate(now),
			transaction.WithAccountingPeriod(now),
			transaction.WithComment(dto.Comment),
			transaction.WithExchangeRate(func() *float64 { rate := dto.GetExchangeRate(); return &rate }()),
			transaction.WithDestinationAmount(destinationAmount),
		)
	} else {
		// Same currency transfer - create a regular TRANSFER transaction
		transferTransaction = transaction.New(
			transferAmount,
			transaction.Transfer,
			transaction.WithTenantID(tenantID),
			transaction.WithOriginAccountID(sourceAccountID),
			transaction.WithDestinationAccountID(destinationAccountID),
			transaction.WithTransactionDate(now),
			transaction.WithAccountingPeriod(now),
			transaction.WithComment(dto.Comment),
		)
	}

	// Create transfer transaction and update both account balances
	err = composables.InTx(r.Context(), func(txCtx context.Context) error {
		_, err := c.transactionService.Create(txCtx, transferTransaction)
		if err != nil {
			return err
		}

		// Recalculate balance for source account
		if err := c.moneyAccountService.RecalculateBalance(txCtx, sourceAccountID); err != nil {
			return err
		}

		// Recalculate balance for destination account
		if err := c.moneyAccountService.RecalculateBalance(txCtx, destinationAccountID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

// accountHistory loads one page of the transactions on either side of the
// account, filtered the same way as the transactions list.
func (c *MoneyAccountController) accountHistory(r *http.Request, accountID uuid.UUID) (*moneyaccounts.TransactionsTabProps, error) {
	values := r.URL.Query()
	values.Set("account", accountID.String())
	paginationParams := composables.UsePaginated(r)
	params := &query.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: query.SortBy{
			Fields: []repo.SortByField[query.Field]{
				{Field: query.FieldTransactionDate, Ascending: false},
				{Field: query.FieldCreatedAt, Ascending: false},
				{Field: query.FieldID, Ascending: false},
			},
		},
		Filters: transactionFilters(values),
	}
	transactions, total, err := c.transactionQuery.FindTransactions(r.Context(), params)
	if err != nil {
		return nil, err
	}

	props := &moneyaccounts.TransactionsTabProps{
		AccountID:    accountID.String(),
		Transactions: transactions,
	}
	if params.Offset+len(transactions) < total {
		next := r.URL.Query()
		next.Set(composables.QueryParamPage, strconv.Itoa(paginationParams.Page+1))
		props.NextURL = fmt.Sprintf("%s/%s/transactions?%s", c.basePath, accountID, next.Encode())
	}
	return props, nil
}

// GetAccountTransactions renders a page of the account history for the
// history filters and for scrolling.
func (c *MoneyAccountController) GetAccountTransactions(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	history, err := c.accountHistory(r, id)
	if err != nil {
		http.Error(w, "Error retrieving account transactions", http.StatusInternalServerError)
		return
	}
	templ.Handler(moneyaccounts.TransactionRows(history), templ.WithStreaming()).ServeHTTP(w, r)
}
