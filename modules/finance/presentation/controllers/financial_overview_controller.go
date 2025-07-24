package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/financial_overview"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type FinancialOverviewController struct {
	app      application.Application
	basePath string
}

func NewFinancialOverviewController(app application.Application) application.Controller {
	return &FinancialOverviewController{
		app:      app,
		basePath: "/finance",
	}
}

func (c *FinancialOverviewController) Key() string {
	return c.basePath
}

func (c *FinancialOverviewController) Register(r *mux.Router) {
	// Register all the existing routes but delegate to this controller
	expenseController := NewExpensesController(c.app)
	paymentController := NewPaymentsController(c.app)
	transactionController := NewTransactionController(c.app)

	// Register the individual controllers (for backwards compatibility during migration)
	expenseController.Register(r)
	paymentController.Register(r)
	transactionController.Register(r)

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
