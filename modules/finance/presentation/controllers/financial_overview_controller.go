// Package controllers provides this package.
package controllers

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/components"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/financial_overview"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type FinancialOverviewController struct {
	basePath               string
	paymentService         *services.PaymentService
	moneyAccountService    *services.MoneyAccountService
	counterpartyService    *services.CounterpartyService
	paymentCategoryService *services.PaymentCategoryService
	transactionService     *services.TransactionService
	balanceService         *services.BalanceService
}

func NewFinancialOverviewController(
	paymentService *services.PaymentService,
	moneyAccountService *services.MoneyAccountService,
	counterpartyService *services.CounterpartyService,
	paymentCategoryService *services.PaymentCategoryService,
	transactionService *services.TransactionService,
	balanceService *services.BalanceService,
) application.Controller {
	return &FinancialOverviewController{
		basePath:               "/finance",
		paymentService:         paymentService,
		moneyAccountService:    moneyAccountService,
		counterpartyService:    counterpartyService,
		paymentCategoryService: paymentCategoryService,
		transactionService:     transactionService,
		balanceService:         balanceService,
	}
}

func (c *FinancialOverviewController) Descriptor() application.ControllerDescriptor {
	overviewPath := c.basePath + "/overview"
	return application.Descriptor("finance.financial_overview", 0, application.Route("", overviewPath)).
		WithNav(
			application.NavNode{
				ID:       "finance.financial_overview",
				Parent:   "finance",
				TitleKey: "NavigationLinks.FinancialOverview",
				Path:     overviewPath,
				Order:    10,
			},
			application.NavNode{
				ID:       "finance.payments",
				Parent:   "finance.financial_overview",
				TitleKey: "NavigationLinks.Payments",
				Path:     overviewPath + "?tab=payments",
				Surfaces: map[application.Surface]application.SurfaceOptions{
					application.SurfaceSpotlight: {},
				},
				Actions: []application.NavAction{{
					ID:       "finance.payments.new",
					TitleKey: "Payments.List.New",
					Path:     overviewPath + "?tab=payments",
				}},
			},
			application.NavNode{
				ID:       "finance.expenses",
				Parent:   "finance.financial_overview",
				TitleKey: "NavigationLinks.Expenses",
				Path:     overviewPath + "?tab=expenses",
				Surfaces: map[application.Surface]application.SurfaceOptions{
					application.SurfaceSpotlight: {},
				},
				Actions: []application.NavAction{{
					ID:       "finance.expenses.new",
					TitleKey: "Expenses.List.New",
					Path:     overviewPath + "?tab=expenses",
				}},
			},
		)
}

func (c *FinancialOverviewController) Register(r *mux.Router) {
	// Register all the existing routes but delegate to this controller
	expenseController := NewExpensesController()
	paymentController := NewPaymentsController(c.paymentService, c.moneyAccountService, c.counterpartyService, c.paymentCategoryService)
	transactionController := NewTransactionController(c.transactionService)

	// Register the underlying tab controllers on the shared finance router.
	expenseController.Register(r)
	paymentController.Register(r)
	transactionController.Register(r)

	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}

	// Register the overview route
	router := r.PathPrefix(c.basePath + "/overview").Subrouter()
	router.Use(commonMiddleware...)
	router.HandleFunc("", c.Index).Methods(http.MethodGet)

	balances := r.PathPrefix(c.basePath + "/balances").Subrouter()
	balances.Use(commonMiddleware...)
	balances.HandleFunc("", c.Balances).Methods(http.MethodGet)
	balances.HandleFunc("/{id:[0-9a-fA-F-]+}", c.AccountBalance).Methods(http.MethodGet)
}

// Balances renders the balance summary shared by the finance pages. Users who
// cannot read debts get an empty fragment instead of an error.
func (c *FinancialOverviewController) Balances(w http.ResponseWriter, r *http.Request) {
	balances, err := c.balanceService.Balances(r.Context())
	if errors.Is(err, composables.ErrForbidden) {
		return
	}
	if err != nil {
		http.Error(w, "Error retrieving balances", http.StatusInternalServerError)
		return
	}
	templ.Handler(
		components.BalanceSummary(mapping.MapViewModels(balances, mappers.BalanceToViewModel)),
		templ.WithStreaming(),
	).ServeHTTP(w, r)
}

// AccountBalance renders one account's balance and the obligations reserving
// it. Users who cannot read debts see only what is on the account.
func (c *FinancialOverviewController) AccountBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := shared.ParseUUID(r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusBadRequest, nil)
		return
	}

	props := components.AccountBalanceProps{}
	balance, err := c.balanceService.AccountBalance(ctx, id)
	switch {
	case errors.Is(err, composables.ErrForbidden):
		account, err := c.moneyAccountService.GetByID(ctx, id)
		if err != nil {
			http.Error(w, "Error retrieving money account", http.StatusInternalServerError)
			return
		}
		props.Balance = &viewmodels.Balance{OnAccounts: account.Balance().Display()}
	case err != nil:
		http.Error(w, "Error retrieving balance", http.StatusInternalServerError)
		return
	default:
		props.Balance = mappers.BalanceToViewModel(balance)
		reserves, err := c.balanceService.Reserves(ctx, id)
		if err != nil {
			http.Error(w, "Error retrieving reserves", http.StatusInternalServerError)
			return
		}
		props.Reserves = make([]*viewmodels.Debt, 0, len(reserves))
		for _, reserve := range reserves {
			counterparty, err := c.counterpartyService.GetByID(ctx, reserve.CounterpartyID())
			if err != nil {
				http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
				return
			}
			props.Reserves = append(props.Reserves, mappers.DebtToViewModel(reserve, counterparty.Name()))
		}
	}
	templ.Handler(components.AccountBalance(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *FinancialOverviewController) Index(w http.ResponseWriter, r *http.Request) {
	// Get active tab from query parameter, default to transactions
	activeTab := r.URL.Query().Get("tab")
	if activeTab == "" {
		activeTab = "transactions"
	}

	props := &financial_overview.IndexPageProps{
		ActiveTab: activeTab,
	}

	templ.Handler(financial_overview.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
}
