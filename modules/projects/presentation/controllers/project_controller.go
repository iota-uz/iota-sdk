package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	financeMappers "github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	financeViewModels "github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/templates/pages/projects"
	projectServices "github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ProjectController struct {
	app                 application.Application
	projectService      *projectServices.ProjectService
	counterpartyService *services.CounterpartyService
	basePath            string
	tableDefinition     table.TableDefinition
}

func NewProjectController(app application.Application) application.Controller {
	basePath := "/projects"

	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &ProjectController{
		app:                 app,
		projectService:      app.Service(projectServices.ProjectService{}).(*projectServices.ProjectService),
		counterpartyService: app.Service(services.CounterpartyService{}).(*services.CounterpartyService),
		basePath:            basePath,
		tableDefinition:     tableDefinition,
	}
}

func (c *ProjectController) Key() string {
	return c.basePath
}

func (c *ProjectController) Register(r *mux.Router) {
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
}

func (c *ProjectController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pageCtx := composables.UsePageCtx(ctx)

	paginationParams := composables.UsePaginated(r)

	limit := paginationParams.Limit
	offset := paginationParams.Offset
	sortBy := []string{"created_at DESC"}

	if search := table.UseSearchQuery(r); search != "" {
		// Note: Search functionality would need to be implemented in the service/repository
		// For now, we'll proceed without search filtering
		_ = search
	}

	entities, err := c.projectService.GetPaginated(ctx, limit, offset, sortBy)
	if err != nil {
		http.Error(w, "Error retrieving projects", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	allEntities, err := c.projectService.GetAll(ctx)
	if err != nil {
		http.Error(w, "Error counting projects", http.StatusInternalServerError)
		return
	}
	total := len(allEntities)

	// Create table definition with localized values (only for full page render)
	var definition table.TableDefinition
	if !htmx.IsHxRequest(r) {
		// Create action for drawer
		createAction := actions.CreateAction(
			pageCtx.T("Projects.List.New"),
			"",
		)
		createAction.Attrs = templ.Attributes{
			"hx-get":    c.basePath + "/new/drawer",
			"hx-target": "#view-drawer",
			"hx-swap":   "innerHTML",
		}

		definition = table.NewTableDefinition(
			pageCtx.T("Projects.Meta.List.Title"),
			c.basePath,
		).
			WithColumns(
				table.Column("name", pageCtx.T("Projects.List.Name")),
				table.Column("description", pageCtx.T("Projects.List.Description")),
				table.Column("counterparty", pageCtx.T("Projects.List.Counterparty")),
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
	viewProjects := mapping.MapViewModels(entities, mappers.ProjectDomainToViewModel)
	rows := make([]table.TableRow, 0, len(viewProjects))

	for _, project := range viewProjects {
		createdAt, err := time.Parse("2006-01-02T15:04:05Z07:00", project.CreatedAt)
		if err != nil {
			createdAt, err = time.Parse("2006-01-02 15:04:05", project.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}
		}

		cells := []templ.Component{
			templ.Raw(project.Name),
			templ.Raw(project.Description),
			templ.Raw(project.CounterpartyName),
			table.DateTime(createdAt),
		}

		row := table.Row(cells...).ApplyOpts(
			table.WithDrawer(fmt.Sprintf("%s/%s/drawer", c.basePath, project.ID)),
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

func (c *ProjectController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	counterparties, err := c.viewModelCounterparties(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &projects.DrawerCreateProps{
		Errors:         map[string]string{},
		Project:        dtos.ProjectCreateDTO{},
		Counterparties: counterparties,
	}
	templ.Handler(projects.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProjectController) GetEditDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.projectService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving project", http.StatusInternalServerError)
		return
	}

	counterparties, err := c.viewModelCounterparties(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &projects.DrawerEditProps{
		Project:        mappers.ProjectDomainToViewModel(entity),
		UpdateData:     mappers.ProjectDomainToViewUpdateModel(entity),
		Counterparties: counterparties,
		Errors:         map[string]string{},
	}
	templ.Handler(projects.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProjectController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.ProjectCreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.IsHxRequest(r) && htmx.Target(r) == "project-create-drawer"

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		counterparties, err := c.viewModelCounterparties(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if isDrawer {
			props := &projects.DrawerCreateProps{
				Errors:         errorsMap,
				Project:        *dto,
				Counterparties: counterparties,
			}
			templ.Handler(projects.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
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

	if err := c.projectService.Create(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto, err := composables.UseForm(&dtos.ProjectUpdateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isDrawer := htmx.Target(r) != "" && htmx.Target(r) != "edit-content"

	if errorsMap, ok := dto.Ok(r.Context()); ok {
		existing, err := c.projectService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving project", http.StatusInternalServerError)
			return
		}

		entity, err := dto.Apply(existing)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := c.projectService.Update(r.Context(), entity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Always redirect to refresh the table
		shared.Redirect(w, r, c.basePath)
	} else {
		entity, err := c.projectService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving project", http.StatusInternalServerError)
			return
		}

		counterparties, err := c.viewModelCounterparties(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if isDrawer {
			props := &projects.DrawerEditProps{
				Project:        mappers.ProjectDomainToViewModel(entity),
				UpdateData:     mappers.ProjectDomainToViewUpdateModel(entity),
				Counterparties: counterparties,
				Errors:         errorsMap,
			}
			templ.Handler(projects.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			http.Error(w, "Edit form not supported - use drawer", http.StatusBadRequest)
		}
	}
}

func (c *ProjectController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.projectService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectController) viewModelCounterparties(r *http.Request) ([]*financeViewModels.Counterparty, error) {
	counterparties, err := c.counterpartyService.GetAll(r.Context())
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(counterparties, financeMappers.CounterpartyToViewModel), nil
}
