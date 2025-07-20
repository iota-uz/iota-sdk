package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type DebtAggregateController struct {
	app                 application.Application
	debtService         *services.DebtService
	counterpartyService *services.CounterpartyService
	basePath            string
	tableDefinition     table.TableDefinition
}

func NewDebtAggregateController(app application.Application) application.Controller {
	basePath := "/finance/debt-aggregates"

	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(false).
		Build()

	return &DebtAggregateController{
		app:                 app,
		debtService:         app.Service(services.DebtService{}).(*services.DebtService),
		counterpartyService: app.Service(services.CounterpartyService{}).(*services.CounterpartyService),
		basePath:            basePath,
		tableDefinition:     tableDefinition,
	}
}

func (c *DebtAggregateController) Key() string {
	return c.basePath
}

func (c *DebtAggregateController) Register(r *mux.Router) {
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
	router.HandleFunc("/{counterparty_id:[0-9a-fA-F-]+}/drawer", c.GetCounterpartyDrawer).Methods(http.MethodGet)
}

func (c *DebtAggregateController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	aggregates, err := c.debtService.GetCounterpartyAggregates(ctx)
	if err != nil {
		http.Error(w, "Error retrieving debt aggregates", http.StatusInternalServerError)
		return
	}

	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		definition = table.NewTableDefinition(
			pageCtx.T("DebtAggregates.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("counterparty", pageCtx.T("DebtAggregates.List.Counterparty")),
				table.Column("total_receivable", pageCtx.T("DebtAggregates.List.TotalReceivable")),
				table.Column("total_payable", pageCtx.T("DebtAggregates.List.TotalPayable")),
				table.Column("outstanding_receivable", pageCtx.T("DebtAggregates.List.OutstandingReceivable")),
				table.Column("outstanding_payable", pageCtx.T("DebtAggregates.List.OutstandingPayable")),
				table.Column("net_amount", pageCtx.T("DebtAggregates.List.NetAmount")),
				table.Column("debt_count", pageCtx.T("DebtAggregates.List.DebtCount")),
			).
			WithInfiniteScroll(false).
			Build()
	} else {
		definition = c.tableDefinition
	}

	rows := make([]table.TableRow, 0, len(aggregates))

	for _, agg := range aggregates {
		counterparty, err := c.counterpartyService.GetByID(ctx, agg.CounterpartyID())
		if err != nil {
			http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
			return
		}

		aggVM := mappers.DebtCounterpartyAggregateToViewModel(agg, counterparty.Name())

		cells := []templ.Component{
			templ.Raw(aggVM.CounterpartyName),
			templ.Raw(aggVM.TotalReceivable),
			templ.Raw(aggVM.TotalPayable),
			templ.Raw(aggVM.TotalOutstandingReceivable),
			templ.Raw(aggVM.TotalOutstandingPayable),
			templ.Raw(aggVM.NetAmount),
			templ.Raw(fmt.Sprintf("%d", aggVM.DebtCount)),
		}

		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(fmt.Sprintf("%s/%s/drawer", c.basePath, aggVM.CounterpartyID)),
		)
		rows = append(rows, row)
	}

	tableData := table.NewTableData().
		WithRows(rows...).
		WithQueryParams(r.URL.Query())

	renderer := table.NewTableRenderer(definition, tableData)

	if htmx.IsHxRequest(r) {
		templ.Handler(renderer.RenderRows(), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(renderer.RenderFull(), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *DebtAggregateController) GetCounterpartyDrawer(w http.ResponseWriter, r *http.Request) {
	counterpartyID, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	debts, err := c.debtService.GetByCounterpartyID(ctx, counterpartyID)
	if err != nil {
		http.Error(w, "Error retrieving debts", http.StatusInternalServerError)
		return
	}

	counterparty, err := c.counterpartyService.GetByID(ctx, counterpartyID)
	if err != nil {
		http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
		return
	}

	debtVMs := mapping.MapViewModels(debts, func(d debt.Debt) *viewmodels.Debt {
		return mappers.DebtToViewModel(d, counterparty.Name())
	})

	// Create simple table for drawer content
	definition := table.NewTableDefinition(
		fmt.Sprintf("%s - %s", pageCtx.T("DebtAggregates.Drawer.Title"), counterparty.Name()),
		"",
	).
		WithColumns(
			table.Column("type", pageCtx.T("Debts.List.Type")),
			table.Column("original_amount", pageCtx.T("Debts.List.OriginalAmount")),
			table.Column("outstanding_amount", pageCtx.T("Debts.List.OutstandingAmount")),
			table.Column("status", pageCtx.T("Debts.List.Status")),
			table.Column("description", pageCtx.T("Debts.List.Description")),
		).
		Build()

	rows := make([]table.TableRow, 0, len(debtVMs))
	for _, debtVM := range debtVMs {
		cells := []templ.Component{
			templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Types.%s", debtVM.Type))),
			templ.Raw(debtVM.OriginalAmountWithCurrency),
			templ.Raw(debtVM.OutstandingAmountWithCurrency),
			templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Statuses.%s", debtVM.Status))),
			templ.Raw(debtVM.Description),
		}
		rows = append(rows, table.Row(cells...))
	}

	tableData := table.NewTableData().WithRows(rows...)
	renderer := table.NewTableRenderer(definition, tableData)

	templ.Handler(renderer.RenderFull(), templ.WithStreaming()).ServeHTTP(w, r)
}