// Package controllers provides this package.
package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
)

type DebtAggregateController struct {
	debtService         *services.DebtService
	counterpartyService *services.CounterpartyService
	basePath            string
}

func NewDebtAggregateController(debtService *services.DebtService, counterpartyService *services.CounterpartyService) application.Controller {
	basePath := "/finance/debt-aggregates"

	return &DebtAggregateController{
		debtService:         debtService,
		counterpartyService: counterpartyService,
		basePath:            basePath,
	}
}

func (c *DebtAggregateController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("finance.debt_aggregate", 0, application.Route("", c.basePath)).
		WithNav(application.NavNode{
			ID:       "finance.debt_aggregate",
			Parent:   "finance",
			TitleKey: "NavigationLinks.DebtAggregates",
			Path:     c.basePath,
			Order:    30,
		})
}

func (c *DebtAggregateController) Register(r *mux.Router) {
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
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/{counterparty_id:[0-9a-fA-F-]+}/drawer", c.GetCounterpartyDrawer).Methods(http.MethodGet)
}

// Filter chip field keys of the counterparty debt summary.
const (
	debtAggFilterCounterparty   = "counterparty"
	debtAggFilterCurrency       = "currency"
	debtAggFilterType           = "type"
	debtAggFilterHasOutstanding = "has_outstanding"
)

// filterRegistry describes the chips of the counterparty summary: the
// counterparty, the currency and the type of the summed debts, and whether a
// counterparty still has something outstanding.
func (c *DebtAggregateController) filterRegistry(
	r *http.Request,
	counterparties []counterparty.Counterparty,
	currencyService *coreservices.CurrencyService,
) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	cpOptions := make([]filterbuilder.Option, 0, len(counterparties))
	for _, cp := range counterparties {
		cpOptions = append(cpOptions, filterbuilder.Opt(cp.ID().String(), cp.Name()))
	}
	var currencies []filterbuilder.Option
	if list, err := currencyService.GetAll(r.Context()); err == nil {
		for _, item := range list {
			currencies = append(currencies, filterbuilder.Opt(string(item.Code()), string(item.Code())))
		}
	}
	types := []filterbuilder.Option{
		filterbuilder.Opt(string(debt.DebtTypeReceivable), pageCtx.T("Debts.Types.RECEIVABLE")),
		filterbuilder.Opt(string(debt.DebtTypePayable), pageCtx.T("Debts.Types.PAYABLE")),
	}
	return filterbuilder.NewRegistry(
		filterbuilder.FieldDef{
			Key: debtAggFilterCounterparty, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("DebtAggregates.List.Counterparty"), Options: cpOptions,
		},
		filterbuilder.FieldDef{
			Key: debtAggFilterCurrency, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("DebtAggregates.List.Filters.Currency"), Options: currencies,
		},
		filterbuilder.FieldDef{
			Key: debtAggFilterType, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("DebtAggregates.List.Filters.Type"), Options: types,
		},
		filterbuilder.FieldDef{
			Key: debtAggFilterHasOutstanding, Type: filterq.FieldTypeBool,
			Label: pageCtx.T("DebtAggregates.List.Filters.HasOutstanding"),
		},
	)
}

func (c *DebtAggregateController) List(
	r *http.Request,
	w http.ResponseWriter,
	currencyService *coreservices.CurrencyService,
) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	// Check permission
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Get pagination parameters
	paginationParams := composables.UsePaginated(r)

	counterparties, err := c.counterpartyService.GetAll(ctx)
	if err != nil {
		http.Error(w, "Error retrieving counterparties", http.StatusInternalServerError)
		return
	}
	names := make(map[uuid.UUID]string, len(counterparties))
	for _, cp := range counterparties {
		names[cp.ID()] = cp.Name()
	}

	registry := c.filterRegistry(r, counterparties, currencyService)
	conditions := registry.Decode(r.URL.Query())
	// Counterparty, currency and type narrow the summed debts; "has
	// outstanding" keeps the counterparties whose summed outstanding amount
	// is not zero.
	filters := filterq.RepoFilters(canonicalCounterparties(conditions), registry.Schema(), map[string]filterq.RepoField[debt.AggregateField]{
		// Compared as text: a garbled id in the URL matches nothing instead of
		// failing the ::uuid cast and the whole page.
		debtAggFilterCounterparty:   {Column: debt.AggregateCounterpartyField, Expr: func(col string) string { return col + "::text" }},
		debtAggFilterCurrency:       {Column: debt.AggregateCurrencyField},
		debtAggFilterType:           {Column: debt.AggregateTypeField},
		debtAggFilterHasOutstanding: {Column: debt.AggregateOutstandingField, Expr: func(col string) string { return col + " <> 0" }},
	}, time.Now())
	barProps := filterbuilder.Props{Registry: registry, Filters: conditions, ID: "debt-aggregate-filters", ShowResetAlways: true}

	aggregates, err := c.debtService.GetCounterpartyAggregates(ctx, filters...)
	if err != nil {
		http.Error(w, "Error retrieving debt aggregates", http.StatusInternalServerError)
		return
	}

	// Always create a proper table definition with columns
	definition := table.NewTableDefinition(
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
		WithFilterBar(filterbuilder.Builder(barProps), filterbuilder.BuilderOOB(barProps)).
		WithInfiniteScroll(false).
		Build()

	rows := make([]table.TableRow, 0, len(aggregates))

	for _, agg := range aggregates {
		name, ok := names[agg.CounterpartyID()]
		if !ok {
			cp, err := c.counterpartyService.GetByID(ctx, agg.CounterpartyID())
			if err != nil {
				http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
				return
			}
			name = cp.Name()
		}

		aggVM := mappers.DebtCounterpartyAggregateToViewModel(agg, name)

		cells := []table.TableCell{
			table.Cell(templ.Raw(aggVM.CounterpartyName), aggVM.CounterpartyName),
			table.Cell(templ.Raw(aggVM.TotalReceivable), aggVM.TotalReceivable),
			table.Cell(templ.Raw(aggVM.TotalPayable), aggVM.TotalPayable),
			table.Cell(templ.Raw(aggVM.TotalOutstandingReceivable), aggVM.TotalOutstandingReceivable),
			table.Cell(templ.Raw(aggVM.TotalOutstandingPayable), aggVM.TotalOutstandingPayable),
			table.Cell(templ.Raw(aggVM.NetAmount), aggVM.NetAmount),
			table.Cell(templ.Raw(fmt.Sprintf("%d", aggVM.DebtCount)), aggVM.DebtCount),
		}

		// The drawer lists the debts this row sums: the row's currency and
		// the side chip of the page.
		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(fmt.Sprintf("%s/%s/drawer?%s", c.basePath, aggVM.CounterpartyID, drawerQuery(conditions, agg.CurrencyCode()).Encode())),
		)
		rows = append(rows, row)
	}

	tableData := table.NewTableData().
		WithRows(rows...).
		WithPagination(paginationParams.Page, paginationParams.Limit, int64(len(aggregates))).
		WithQueryParams(r.URL.Query())

	renderer := table.NewTableRenderer(definition, tableData)

	// A history restore (cache miss on Back) needs the whole document.
	if !htmx.WantsFullPage(r) {
		templ.Handler(renderer.RenderRows(), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(renderer.RenderFull(), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *DebtAggregateController) GetCounterpartyDrawer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check permission
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	counterpartyIDStr := mux.Vars(r)["counterparty_id"]
	counterpartyID, err := uuid.Parse(counterpartyIDStr)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	pageCtx := composables.UsePageCtx(ctx)

	debts, err := c.debtService.GetByCounterpartyID(ctx, counterpartyID)
	if err != nil {
		http.Error(w, "Error retrieving debts", http.StatusInternalServerError)
		return
	}
	// Only the debts the summary row was summed from: its currency and the
	// side chip carried by the row's link.
	debts = drawerDebts(debts, r.URL.Query())

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
			table.Column("description", pageCtx.T("Debts.List._Description")),
		).
		Build()

	rows := make([]table.TableRow, 0, len(debtVMs))
	for _, debtVM := range debtVMs {
		cells := []table.TableCell{
			table.Cell(templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Types.%s", debtVM.Type))), debtVM.Type),
			table.Cell(templ.Raw(debtVM.OriginalAmountWithCurrency), debtVM.OriginalAmountWithCurrency),
			table.Cell(templ.Raw(debtVM.OutstandingAmountWithCurrency), debtVM.OutstandingAmountWithCurrency),
			table.Cell(templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Statuses.%s", debtVM.Status))), debtVM.Status),
			table.Cell(templ.Raw(debtVM.Description), debtVM.Description),
		}
		rows = append(rows, table.Row(cells...))
	}

	tableData := table.NewTableData().WithRows(rows...)
	renderer := table.NewTableRenderer(definition, tableData)

	// Create a drawer wrapper for the counterparty debt details table
	drawerProps := table.DefaultDrawerProps{
		Title:       fmt.Sprintf("%s - %s", pageCtx.T("DebtAggregates.Drawer.Title"), counterparty.Name()),
		CallbackURL: c.basePath,
	}

	// Create a custom component that renders the drawer with table content
	drawerWithTable := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return table.DefaultDrawer(drawerProps).Render(templ.WithChildren(ctx, renderer.RenderTable()), w)
	})

	templ.Handler(drawerWithTable, templ.WithStreaming()).ServeHTTP(w, r)
}

// canonicalCounterparties lowercases the counterparty ids of the conditions:
// the column is compared as text, and PostgreSQL prints a UUID in lowercase,
// so an id typed in uppercase would otherwise stop matching. A malformed value
// stays malformed and still matches nothing.
func canonicalCounterparties(conditions filterq.FilterSet) filterq.FilterSet {
	out := make(filterq.FilterSet, len(conditions))
	for i, c := range conditions {
		if c.Field == debtAggFilterCounterparty {
			values := make([]string, len(c.Values))
			for j, v := range c.Values {
				values[j] = strings.ToLower(v)
			}
			c.Values = values
		}
		out[i] = c
	}
	return out
}

// drawerQuery is what a summary row passes to its drawer: the row's currency
// and the side chip, the conditions that narrow which debts are summed.
// "Has outstanding" filters the summed rows, not the debts, so it stays out.
func drawerQuery(conditions filterq.FilterSet, currency string) url.Values {
	kept := filterq.FilterSet{{Field: debtAggFilterCurrency, Op: filterq.OpIs, Values: []string{currency}}}
	for _, c := range conditions {
		if c.Field == debtAggFilterType {
			kept = append(kept, c)
		}
	}
	return filterq.Encode(kept)
}

// drawerDebts keeps the debts matching the currency and side conditions of
// the drawer link (drawerQuery).
func drawerDebts(debts []debt.Debt, q url.Values) []debt.Debt {
	var conditions filterq.FilterSet
	for _, raw := range q[filterq.ParamName] {
		if c, ok := filterq.DecodeCondition(raw); ok && (c.Field == debtAggFilterCurrency || c.Field == debtAggFilterType) {
			conditions = append(conditions, c)
		}
	}
	out := make([]debt.Debt, 0, len(debts))
	for _, d := range debts {
		keep := true
		for _, c := range conditions {
			switch c.Field {
			case debtAggFilterCurrency:
				keep = keep && d.OriginalAmount() != nil && c.MatchValue(d.OriginalAmount().Currency().Code)
			case debtAggFilterType:
				keep = keep && c.MatchValue(string(d.Type()))
			}
		}
		if keep {
			out = append(out, d)
		}
	}
	return out
}
