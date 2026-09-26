// Package controllers provides this package.
package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/debts"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type DebtsController struct {
	debtService         *services.DebtService
	counterpartyService *services.CounterpartyService
	moneyAccountService *services.MoneyAccountService
	currencyService     *coreservices.CurrencyService
	projectDirectory    services.ProjectDirectory
	basePath            string
	tableDefinition     table.TableDefinition
}

func NewDebtsController(
	debtService *services.DebtService,
	counterpartyService *services.CounterpartyService,
	moneyAccountService *services.MoneyAccountService,
	currencyService *coreservices.CurrencyService,
	projectDirectory services.ProjectDirectory,
) application.Controller {
	basePath := "/finance/debts"

	// Create table definition with columns for HTMX requests
	tableDefinition := table.NewTableDefinition("", basePath).
		WithColumns(
			table.Column("counterparty", "Counterparty"),
			table.Column("type", "Type"),
			table.Column("original_amount", "Original Amount"),
			table.Column("outstanding_amount", "Outstanding Amount"),
			table.Column("status", "Status"),
			table.Column("description", "Description"),
			table.Column("due_date", "Due Date"),
			table.Column("created_at", "Created At"),
		).
		WithInfiniteScroll(true).
		Build()

	return &DebtsController{
		debtService:         debtService,
		counterpartyService: counterpartyService,
		moneyAccountService: moneyAccountService,
		currencyService:     currencyService,
		projectDirectory:    projectDirectory,
		basePath:            basePath,
		tableDefinition:     tableDefinition,
	}
}

func (c *DebtsController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("finance.debt", 0, application.Route("", c.basePath)).
		WithNav(application.NavNode{
			ID:       "finance.debt",
			Parent:   "finance",
			TitleKey: "NavigationLinks.Debts",
			Path:     c.basePath,
			Order:    20,
		})
}

func (c *DebtsController) Register(r *mux.Router) {
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
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/settle", c.Settle).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/write-off", c.WriteOff).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/cancel", c.Cancel).Methods(http.MethodPost)
}

func (c *DebtsController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	// Check permission
	if err := composables.CanUser(ctx, permissions.DebtRead); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	paginationParams := composables.UsePaginated(r)
	params := &debt.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}

	if search := table.UseSearchQuery(r); search != "" {
		params.Query = search
	}

	if from := r.URL.Query().Get("CreatedAt.From"); from != "" {
		if to := r.URL.Query().Get("CreatedAt.To"); to != "" {
			params.CreatedAt = debt.DateRange{
				From: from,
				To:   to,
			}
		}
	}

	debtEntities, err := c.debtService.GetPaginated(ctx, params)
	if err != nil {
		http.Error(w, "Error retrieving debts", http.StatusInternalServerError)
		return
	}

	total, err := c.debtService.Count(ctx)
	if err != nil {
		http.Error(w, "Error counting debts", http.StatusInternalServerError)
		return
	}

	// Create table definition with localized values (only for full page render)
	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		// Create action for drawer
		createAction := actions.CreateAction(
			pageCtx.T("Debts.List.New"),
			"",
		)
		createAction.Attrs = templ.Attributes{
			"hx-get":    c.basePath + "/new/drawer",
			"hx-target": "#view-drawer",
			"hx-swap":   "innerHTML",
		}

		definition = table.NewTableDefinition(
			pageCtx.T("Debts.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("counterparty", pageCtx.T("Debts.List.Counterparty")),
				table.Column("type", pageCtx.T("Debts.List.Type")),
				table.Column("original_amount", pageCtx.T("Debts.List.OriginalAmount")),
				table.Column("outstanding_amount", pageCtx.T("Debts.List.OutstandingAmount")),
				table.Column("status", pageCtx.T("Debts.List.Status")),
				table.Column("description", pageCtx.T("Debts.List._Description")),
				table.Column("due_date", pageCtx.T("Debts.List.DueDate")),
				table.Column("created_at", pageCtx.T("CreatedAt")),
			).
			WithActions(actions.RenderAction(createAction)).
			WithFilters(filters.CreatedAt()).
			WithDeferredPanels(table.DeferredPanel{ID: "debt-balances", URL: "/finance/balances"}).
			WithInfiniteScroll(true).
			Build()
	} else {
		// For HTMX requests, use minimal definition
		definition = c.tableDefinition
	}

	// Build table rows
	rows := make([]table.TableRow, 0, len(debtEntities))

	for _, d := range debtEntities {
		// Get counterparty name
		counterparty, err := c.counterpartyService.GetByID(ctx, d.CounterpartyID())
		if err != nil {
			http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
			return
		}

		debtVM := mappers.DebtToViewModel(d, counterparty.Name())

		createdAt, err := time.Parse("2006-01-02T15:04:05Z07:00", debtVM.CreatedAt)
		if err != nil {
			createdAt, err = time.Parse("2006-01-02 15:04:05", debtVM.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}
		}

		cells := []table.TableCell{
			table.Cell(templ.Raw(debtVM.CounterpartyName), debtVM.CounterpartyName),
			table.Cell(templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Types.%s", debtVM.Type))), debtVM.Type),
			table.Cell(templ.Raw(debtVM.OriginalAmountWithCurrency), debtVM.OriginalAmount),
			table.Cell(templ.Raw(debtVM.OutstandingAmountWithCurrency), debtVM.OutstandingAmount),
			table.Cell(templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Statuses.%s", debtVM.Status))), debtVM.Status),
			table.Cell(templ.Raw(templ.EscapeString(debtVM.Description)), debtVM.Description),
			table.Cell(templ.Raw(debtVM.DueDate), debtVM.DueDate),
			table.Cell(table.DateTime(createdAt), createdAt),
		}

		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(fmt.Sprintf("%s/%s/drawer", c.basePath, debtVM.ID)),
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

func (c *DebtsController) drawerOptions(ctx context.Context) (debts.DrawerOptions, error) {
	counterparties, err := c.counterpartyService.GetAll(ctx)
	if err != nil {
		return debts.DrawerOptions{}, fmt.Errorf("retrieving counterparties: %w", err)
	}
	currencies, err := c.currencyService.GetAll(ctx)
	if err != nil {
		return debts.DrawerOptions{}, fmt.Errorf("retrieving currencies: %w", err)
	}
	accounts, err := c.moneyAccountService.GetAll(ctx)
	if err != nil {
		return debts.DrawerOptions{}, fmt.Errorf("retrieving accounts: %w", err)
	}
	projects, err := c.projectDirectory.Projects(ctx)
	if err != nil {
		return debts.DrawerOptions{}, fmt.Errorf("retrieving projects: %w", err)
	}
	return debts.DrawerOptions{
		Counterparties: mapping.MapViewModels(counterparties, mappers.CounterpartyToViewModel),
		Currencies:     mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel),
		Accounts:       mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel),
		Projects:       mapping.MapViewModels(projects, mappers.ProjectRefToViewModel),
	}, nil
}

func (c *DebtsController) debtViewModel(ctx context.Context, entity debt.Debt) (*viewmodels.Debt, error) {
	counterparty, err := c.counterpartyService.GetByID(ctx, entity.CounterpartyID())
	if err != nil {
		return nil, fmt.Errorf("retrieving counterparty: %w", err)
	}
	return mappers.DebtToViewModel(entity, counterparty.Name()), nil
}

func (c *DebtsController) renderEditDrawer(w http.ResponseWriter, r *http.Request, props *debts.DrawerEditProps) {
	options, err := c.drawerOptions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props.Options = options
	templ.Handler(debts.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

// withFormValues shows what the user typed instead of the saved values, so a
// rejected form keeps its input.
func withFormValues(vm *viewmodels.Debt, dto *dtos.DebtUpdateDTO) *viewmodels.Debt {
	vm.CounterpartyID = dto.CounterpartyID
	vm.Type = dto.Type
	vm.OriginalAmount = fmt.Sprintf("%.2f", dto.Amount)
	vm.CurrencyCode = dto.CurrencyCode
	vm.MoneyAccountID = dto.MoneyAccountID
	vm.ProjectID = dto.ProjectID
	vm.Description = dto.Description
	vm.DueDate = ""
	if !time.Time(dto.DueDate).IsZero() {
		vm.DueDate = time.Time(dto.DueDate).Format(time.DateOnly)
	}
	return vm
}

// accountError turns a debt/account currency mismatch into a form error.
func accountError(ctx context.Context, err error) (map[string]string, bool) {
	if !errors.Is(err, services.ErrDebtAccountCurrency) {
		return nil, false
	}
	pageCtx := composables.UsePageCtx(ctx)
	return map[string]string{"MoneyAccountID": pageCtx.T("Debts.Errors.AccountCurrency")}, true
}

func (c *DebtsController) GetEditDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.debtService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving debt", http.StatusInternalServerError)
		return
	}
	vm, err := c.debtViewModel(r.Context(), entity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	c.renderEditDrawer(w, r, &debts.DrawerEditProps{Debt: vm, Errors: map[string]string{}})
}

func (c *DebtsController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	options, err := c.drawerOptions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &debts.DrawerCreateProps{
		Errors:  map[string]string{},
		Debt:    dtos.DebtCreateDTO{},
		Options: options,
	}
	templ.Handler(debts.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *DebtsController) renderCreateDrawer(w http.ResponseWriter, r *http.Request, dto *dtos.DebtCreateDTO, errorsMap map[string]string) {
	options, err := c.drawerOptions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &debts.DrawerCreateProps{
		Errors:  errorsMap,
		Debt:    *dto,
		Options: options,
	}
	templ.Handler(debts.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *DebtsController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.DebtCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.IsHxRequest(r) && htmx.Target(r) == "debt-create-drawer"

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		if isDrawer {
			c.renderCreateDrawer(w, r, dto, errorsMap)
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

	entity := dto.ToEntity(tenantID)
	if _, err := c.debtService.Create(r.Context(), entity); err != nil {
		if errorsMap, ok := accountError(r.Context(), err); ok && isDrawer {
			c.renderCreateDrawer(w, r, dto, errorsMap)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *DebtsController) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check permission
	if err := composables.CanUser(ctx, permissions.DebtUpdate); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.DebtUpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existing, err := c.debtService.GetByID(ctx, id)
	if err != nil {
		http.Error(w, "Error retrieving debt", http.StatusInternalServerError)
		return
	}

	errorsMap, ok := dto.Ok(ctx)
	if ok {
		entity, err := dto.Apply(existing)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = c.debtService.Update(ctx, entity)
		if err == nil {
			shared.Redirect(w, r, c.basePath)
			return
		}
		if errorsMap, ok = accountError(ctx, err); !ok {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	vm, err := c.debtViewModel(ctx, existing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.renderEditDrawer(w, r, &debts.DrawerEditProps{Debt: withFormValues(vm, dto), Errors: errorsMap})
}

func (c *DebtsController) Settle(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.DebtSettleDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.debtService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving debt", http.StatusInternalServerError)
			return
		}
		vm, err := c.debtViewModel(r.Context(), entity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		c.renderEditDrawer(w, r, &debts.DrawerEditProps{
			Debt:             vm,
			SettlementAmount: r.FormValue("SettlementAmount"),
			Errors:           errorsMap,
		})
		return
	}

	settlementTransactionID := dto.GetTransactionID()

	if _, err := c.debtService.Settle(r.Context(), id, dto.SettlementAmount, settlementTransactionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *DebtsController) WriteOff(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.debtService.WriteOff(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *DebtsController) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.debtService.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, services.ErrDebtNotOpen) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *DebtsController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.debtService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
