package controllers

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/templates/pages/project_stages"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ProjectStageController struct {
	app             application.Application
	basePath        string
	tableDefinition table.TableDefinition
}

func NewProjectStageController(app application.Application) application.Controller {
	basePath := "/project-stages"

	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &ProjectStageController{
		app:             app,
		basePath:        basePath,
		tableDefinition: tableDefinition,
	}
}

func (c *ProjectStageController) Key() string {
	return c.basePath
}

func (c *ProjectStageController) Register(r *mux.Router) {
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
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", di.H(c.GetEditDrawer)).Methods(http.MethodGet)
	router.HandleFunc("/new/drawer", di.H(c.GetNewDrawer)).Methods(http.MethodGet)
	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", di.H(c.Delete)).Methods(http.MethodDelete)

	// Additional route for stages by project
	router.HandleFunc("/by-project/{projectId:[0-9a-fA-F-]+}", di.H(c.ListByProject)).Methods(http.MethodGet)
}

func (c *ProjectStageController) List(
	r *http.Request,
	w http.ResponseWriter,
	projectStageService *services.ProjectStageService,
) {
	paginationParams := composables.UsePaginated(r)

	entities, err := projectStageService.GetPaginated(r.Context(), paginationParams.Limit, paginationParams.Offset, []string{})
	if err != nil {
		http.Error(w, "Failed to fetch project stages", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := projectStageService.Count(r.Context())
	if err != nil {
		http.Error(w, "Failed to count project stages", http.StatusInternalServerError)
		return
	}

	viewModels := mappers.ProjectStageDomainToViewModels(entities)

	props := &project_stages.IndexPageProps{
		ProjectStages:   viewModels,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), paginationParams.Limit),
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(project_stages.ProjectStagesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(project_stages.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ProjectStageController) ListByProject(
	r *http.Request,
	w http.ResponseWriter,
	projectStageService *services.ProjectStageService,
) {
	vars := mux.Vars(r)
	projectID, err := uuid.Parse(vars["projectId"])
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	entities, err := projectStageService.GetByProjectID(r.Context(), projectID)
	if err != nil {
		http.Error(w, "Failed to fetch project stages", http.StatusInternalServerError)
		return
	}

	viewModels := mappers.ProjectStageDomainToViewModels(entities)

	// For project-filtered views, we don't need pagination since it's typically a smaller set
	props := &project_stages.IndexPageProps{
		ProjectStages:   viewModels,
		PaginationState: nil,
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(project_stages.ProjectStagesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(project_stages.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *ProjectStageController) GetNewDrawer(
	r *http.Request,
	w http.ResponseWriter,
	projectService *services.ProjectService,
) {
	// Get available projects
	projects, err := projectService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
		return
	}

	projectViewModels := mapping.MapViewModels(projects, mappers.ProjectDomainToViewModel)

	// Check if a project ID is provided as query parameter
	dto := dtos.ProjectStageCreateDTO{}
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		if _, err := uuid.Parse(projectID); err == nil {
			dto.ProjectID = projectID
		}
	}

	props := &project_stages.DrawerCreateProps{
		ProjectStage: dto,
		Projects:     projectViewModels,
		Errors:       map[string]string{},
	}
	templ.Handler(project_stages.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProjectStageController) GetEditDrawer(
	r *http.Request,
	w http.ResponseWriter,
	projectStageService *services.ProjectStageService,
) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project stage ID", http.StatusBadRequest)
		return
	}

	entity, err := projectStageService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Project stage not found", http.StatusNotFound)
		return
	}

	viewModel := mappers.ProjectStageDomainToViewModel(entity)

	props := &project_stages.DrawerEditProps{
		ProjectStage: &viewModel,
		Errors:       map[string]string{},
	}
	templ.Handler(project_stages.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProjectStageController) Create(
	r *http.Request,
	w http.ResponseWriter,
	projectStageService *services.ProjectStageService,
	projectService *services.ProjectService,
) {
	dto, err := composables.UseForm(&dtos.ProjectStageCreateDTO{}, r)
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		// Get available projects for re-rendering the form
		projects, err := projectService.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Failed to fetch projects", http.StatusInternalServerError)
			return
		}

		projectViewModels := mapping.MapViewModels(projects, mappers.ProjectDomainToViewModel)

		props := &project_stages.DrawerCreateProps{
			ProjectStage: *dto,
			Projects:     projectViewModels,
			Errors:       errorsMap,
		}
		templ.Handler(project_stages.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity := dto.ToEntity()

	if err := projectStageService.Create(r.Context(), entity); err != nil {
		http.Error(w, "Failed to save project stage", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectStageController) Update(
	r *http.Request,
	w http.ResponseWriter,
	projectStageService *services.ProjectStageService,
) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project stage ID", http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&dtos.ProjectStageUpdateDTO{}, r)
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	existing, err := projectStageService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Project stage not found", http.StatusNotFound)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		viewModel := mappers.ProjectStageDomainToViewModel(existing)

		props := &project_stages.DrawerEditProps{
			ProjectStage: &viewModel,
			Errors:       errorsMap,
		}
		templ.Handler(project_stages.EditDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	updated := dto.Apply(existing)

	if err := projectStageService.Update(r.Context(), updated); err != nil {
		http.Error(w, "Failed to save project stage", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectStageController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	projectStageService *services.ProjectStageService,
) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project stage ID", http.StatusBadRequest)
		return
	}

	_, err = projectStageService.Delete(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to delete project stage", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectStageController) getListParams(r *http.Request) struct {
	Limit  int
	Offset int
	SortBy []string
} {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	var sortBy []string
	if s := r.URL.Query().Get("sort"); s != "" {
		sortBy = []string{s}
	}

	return struct {
		Limit  int
		Offset int
		SortBy []string
	}{
		Limit:  limit,
		Offset: offset,
		SortBy: sortBy,
	}
}
