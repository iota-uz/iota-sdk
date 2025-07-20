package controllers

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/templates/pages/project_stages"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ProjectStageController struct {
	app                 application.Application
	projectStageService *services.ProjectStageService
	basePath            string
	tableDefinition     table.TableDefinition
}

func NewProjectStageController(app application.Application) application.Controller {
	basePath := "/project-stages"

	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &ProjectStageController{
		app:                 app,
		projectStageService: app.Service(services.ProjectStageService{}).(*services.ProjectStageService),
		basePath:            basePath,
		tableDefinition:     tableDefinition,
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
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/drawer", c.GetEditDrawer).Methods(http.MethodGet)
	router.HandleFunc("/new/drawer", c.GetNewDrawer).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)

	// Additional route for stages by project
	router.HandleFunc("/by-project/{projectId:[0-9a-fA-F-]+}", c.ListByProject).Methods(http.MethodGet)
}

func (c *ProjectStageController) List(w http.ResponseWriter, r *http.Request) {
	params := c.getListParams(r)

	entities, err := c.projectStageService.GetPaginated(r.Context(), params.Limit, params.Offset, params.SortBy)
	if err != nil {
		http.Error(w, "Failed to fetch project stages", http.StatusInternalServerError)
		return
	}

	_ = mappers.ProjectStageDomainToViewModels(entities)

	if htmx.IsHxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Project stages table - templates not implemented yet"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message": "Project stages list - templates not implemented yet"}`))
}

func (c *ProjectStageController) ListByProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID, err := uuid.Parse(vars["projectId"])
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	entities, err := c.projectStageService.GetByProjectID(r.Context(), projectID)
	if err != nil {
		http.Error(w, "Failed to fetch project stages", http.StatusInternalServerError)
		return
	}

	_ = mappers.ProjectStageDomainToViewModels(entities)

	if htmx.IsHxRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Project stages by project table - templates not implemented yet"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message": "Project stages by project list - templates not implemented yet"}`))
}

func (c *ProjectStageController) GetNewDrawer(w http.ResponseWriter, r *http.Request) {
	props := &project_stages.DrawerCreateProps{
		ProjectStage: dtos.ProjectStageCreateDTO{},
		Errors:       map[string]string{},
	}
	templ.Handler(project_stages.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ProjectStageController) GetEditDrawer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project stage ID", http.StatusBadRequest)
		return
	}

	entity, err := c.projectStageService.GetByID(r.Context(), id)
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

func (c *ProjectStageController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.ProjectStageCreateDTO{}, r)
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &project_stages.DrawerCreateProps{
			ProjectStage: *dto,
			Errors:       errorsMap,
		}
		templ.Handler(project_stages.CreateDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity := dto.ToEntity()

	if err := c.projectStageService.Create(r.Context(), entity); err != nil {
		http.Error(w, "Failed to save project stage", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectStageController) Update(w http.ResponseWriter, r *http.Request) {
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

	existing, err := c.projectStageService.GetByID(r.Context(), id)
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

	if err := c.projectStageService.Update(r.Context(), updated); err != nil {
		http.Error(w, "Failed to save project stage", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *ProjectStageController) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project stage ID", http.StatusBadRequest)
		return
	}

	_, err = c.projectStageService.Delete(r.Context(), id)
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
