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
	app                    application.Application
	basePath               string
	paymentService         *services.PaymentService
	moneyAccountService    *services.MoneyAccountService
	counterpartyService    *services.CounterpartyService
	paymentCategoryService *services.PaymentCategoryService
	transactionService     *services.TransactionService
}

func NewFinancialOverviewController(
	app application.Application,
	paymentService *services.PaymentService,
	moneyAccountService *services.MoneyAccountService,
	counterpartyService *services.CounterpartyService,
	paymentCategoryService *services.PaymentCategoryService,
	transactionService *services.TransactionService,
) application.Controller {
	return &FinancialOverviewController{
		app:                    app,
		basePath:               "/finance",
		paymentService:         paymentService,
		moneyAccountService:    moneyAccountService,
		counterpartyService:    counterpartyService,
		paymentCategoryService: paymentCategoryService,
		transactionService:     transactionService,
	}
}

func (c *FinancialOverviewController) Key() string {
	return c.basePath
}

func (c *FinancialOverviewController) Register(r *mux.Router) {
	// Register all the existing routes but delegate to this controller
	expenseController := NewExpensesController(c.app)
	paymentController := NewPaymentsController(c.app, c.paymentService, c.moneyAccountService, c.counterpartyService, c.paymentCategoryService)
	transactionController := NewTransactionController(c.app, c.transactionService)

	// Register the underlying tab controllers on the shared finance router.
	expenseController.Register(r)
	paymentController.Register(r)
	transactionController.Register(r)

	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.ProvideLocalizer(c.app),
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
