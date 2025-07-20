package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/debts"
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
	app                 application.Application
	debtService         *services.DebtService
	counterpartyService *services.CounterpartyService
	transactionService  *services.TransactionService
	basePath            string
	tableDefinition     table.TableDefinition
}

func NewDebtsController(app application.Application) application.Controller {
	basePath := "/finance/debts"

	// Create table definition once at initialization
	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &DebtsController{
		app:                 app,
		debtService:         app.Service(services.DebtService{}).(*services.DebtService),
		counterpartyService: app.Service(services.CounterpartyService{}).(*services.CounterpartyService),
		transactionService:  app.Service(services.TransactionService{}).(*services.TransactionService),
		basePath:            basePath,
		tableDefinition:     tableDefinition,
	}
}

func (c *DebtsController) Key() string {
	return c.basePath
}

func (c *DebtsController) Register(r *mux.Router) {
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
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", c.GetEditDrawer).Methods(http.MethodGet)
	router.HandleFunc("/new/drawer", c.GetNewDrawer).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/settle", c.Settle).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/write-off", c.WriteOff).Methods(http.MethodPost)
}

func (c *DebtsController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

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
				table.Column("created_at", pageCtx.T("CreatedAt")),
			).
			WithActions(actions.RenderAction(createAction)).
			WithFilters(filters.CreatedAt()).
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

		cells := []templ.Component{
			templ.Raw(debtVM.CounterpartyName),
			templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Types.%s", debtVM.Type))),
			templ.Raw(debtVM.OriginalAmount),
			templ.Raw(debtVM.OutstandingAmount),
			templ.Raw(pageCtx.T(fmt.Sprintf("Debts.Statuses.%s", debtVM.Status))),
			table.DateTime(createdAt),
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

	// Get counterparty name
	counterparty, err := c.counterpartyService.GetByID(r.Context(), entity.CounterpartyID())
	if err != nil {
		http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
		return
	}

	// Get all counterparties for dropdown
	counterparties, err := c.counterpartyService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving counterparties", http.StatusInternalServerError)
		return
	}

	props := &debts.DrawerEditProps{
		Debt:           mappers.DebtToViewModel(entity, counterparty.Name()),
		Counterparties: mapping.MapViewModels(counterparties, mappers.CounterpartyToViewModel),
		Errors:         map[string]string{},
	}
	templ.Handler(debts.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *DebtsController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	// Get all counterparties for dropdown
	counterparties, err := c.counterpartyService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving counterparties", http.StatusInternalServerError)
		return
	}

	props := &debts.DrawerCreateProps{
		Errors:         map[string]string{},
		Debt:           dtos.DebtCreateDTO{},
		Counterparties: mapping.MapViewModels(counterparties, mappers.CounterpartyToViewModel),
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
			// Get counterparties for dropdown
			counterparties, err := c.counterpartyService.GetAll(r.Context())
			if err != nil {
				http.Error(w, "Error retrieving counterparties", http.StatusInternalServerError)
				return
			}

			props := &debts.DrawerCreateProps{
				Errors:         errorsMap,
				Debt:           *dto,
				Counterparties: mapping.MapViewModels(counterparties, mappers.CounterpartyToViewModel),
			}
			templ.Handler(debts.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *DebtsController) Update(w http.ResponseWriter, r *http.Request) {
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

	isDrawer := htmx.Target(r) != "" && htmx.Target(r) != "edit-content"

	if errorsMap, ok := dto.Ok(r.Context()); ok {
		existing, err := c.debtService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving debt", http.StatusInternalServerError)
			return
		}

		entity, err := dto.Apply(existing)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := c.debtService.Update(r.Context(), entity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Always redirect to refresh the table
		shared.Redirect(w, r, c.basePath)
	} else {
		entity, err := c.debtService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving debt", http.StatusInternalServerError)
			return
		}

		// Get counterparty name
		counterparty, err := c.counterpartyService.GetByID(r.Context(), entity.CounterpartyID())
		if err != nil {
			http.Error(w, "Error retrieving counterparty", http.StatusInternalServerError)
			return
		}

		// Get all counterparties for dropdown
		counterparties, err := c.counterpartyService.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Error retrieving counterparties", http.StatusInternalServerError)
			return
		}

		if isDrawer {
			props := &debts.DrawerEditProps{
				Debt:           mappers.DebtToViewModel(entity, counterparty.Name()),
				Counterparties: mapping.MapViewModels(counterparties, mappers.CounterpartyToViewModel),
				Errors:         errorsMap,
			}
			templ.Handler(debts.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Edit form not supported - use drawer", http.StatusBadRequest)
		}
	}
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
		http.Error(w, fmt.Sprintf("Validation errors: %v", errorsMap), http.StatusBadRequest)
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
