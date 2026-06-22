// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/financial_overview"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type FinancialOverviewController struct {
	basePath               string
	paymentService         *services.PaymentService
	moneyAccountService    *services.MoneyAccountService
	counterpartyService    *services.CounterpartyService
	paymentCategoryService *services.PaymentCategoryService
	transactionService     *services.TransactionService
}

func NewFinancialOverviewController(
	paymentService *services.PaymentService,
	moneyAccountService *services.MoneyAccountService,
	counterpartyService *services.CounterpartyService,
	paymentCategoryService *services.PaymentCategoryService,
	transactionService *services.TransactionService,
) application.Controller {
	return &FinancialOverviewController{
		basePath:               "/finance",
		paymentService:         paymentService,
		moneyAccountService:    moneyAccountService,
		counterpartyService:    counterpartyService,
		paymentCategoryService: paymentCategoryService,
		transactionService:     transactionService,
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
