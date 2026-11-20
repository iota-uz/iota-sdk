// Package controllers provides this package.
package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/templates/pages/project_stages"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ProjectStageController struct {
	basePath        string
	tableDefinition table.TableDefinition
}

func NewProjectStageController() application.Controller {
	basePath := "/project-stages"

	tableDefinition := table.NewTableDefinition("", basePath).
		WithInfiniteScroll(true).
		Build()

	return &ProjectStageController{
		basePath:        basePath,
		tableDefinition: tableDefinition,
	}
}

func (c *ProjectStageController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("projects.project_stage", 0, application.Route("", c.basePath)).
		WithNav(application.NavNode{
			ID:       "projects.project_stage",
			Parent:   "projects",
			TitleKey: "NavigationLinks.ProjectStages",
			Path:     c.basePath,
			Actions: []application.NavAction{{
				ID:       "projects.project_stage.new",
				TitleKey: "ProjectStages.List.New",
				Path:     c.basePath + "/new",
			}},
		})
}

func (c *ProjectStageController) Register(r *mux.Router) {
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
	projectService *services.ProjectService,
) {
	paginationParams := composables.UsePaginated(r)

	registry := c.filterRegistry(r, projectService)
	// Links made by the former created-at filter still work.
	conditions := registry.Decode(filterq.LegacyDateRange(r.URL.Query(), "CreatedAt.From", "CreatedAt.To", "created_at"))
	params := &projectstage.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		Search: strings.TrimSpace(r.URL.Query().Get("Search")),
		Filters: filterq.RepoFilters(conditions, registry.Schema(), map[string]filterq.RepoField[projectstage.Field]{
			"project":     {Column: projectstage.ProjectID, Type: "uuid"},
			"start":       {Column: projectstage.StartDate},
			"planned_end": {Column: projectstage.PlannedEndDate},
			"factual_end": {Column: projectstage.FactualEndDate},
			"created_at":  {Column: projectstage.CreatedAt},
		}, time.Now()),
	}

	entities, err := projectStageService.Find(r.Context(), params)
	if err != nil {
		http.Error(w, "Failed to fetch project stages", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := projectStageService.CountFound(r.Context(), params)
	if err != nil {
		http.Error(w, "Failed to count project stages", http.StatusInternalServerError)
		return
	}

	viewModels := mappers.ProjectStageDomainToViewModels(entities)

	props := &project_stages.IndexPageProps{
		ProjectStages:   viewModels,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), paginationParams.Limit),
		Filters: filterbuilder.Props{
			Registry: registry, Filters: conditions, ID: "project-stage-filters", ShowResetAlways: true,
		},
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(project_stages.ProjectStagesTableWithChips(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(project_stages.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

// filterRegistry describes the chips of the project stages list: the project
// and the stage dates; a stage without a factual end date is not finished.
func (c *ProjectStageController) filterRegistry(r *http.Request, projectService *services.ProjectService) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	var projects []filterbuilder.Option
	if all, err := projectService.GetAll(r.Context()); err == nil {
		for _, p := range all {
			projects = append(projects, filterbuilder.Opt(p.ID().String(), p.Name()))
		}
	}
	return filterbuilder.NewRegistry(
		filterbuilder.FieldDef{Key: "project", Type: filterq.FieldTypeReference, Label: pageCtx.T("ProjectStages.Single.ProjectID"), Options: projects},
		filterbuilder.FieldDef{Key: "start", Type: filterq.FieldTypeDate, Label: pageCtx.T("ProjectStages.List.StartDate"), Nullable: true},
		filterbuilder.FieldDef{Key: "planned_end", Type: filterq.FieldTypeDate, Label: pageCtx.T("ProjectStages.List.PlannedEndDate"), Nullable: true},
		filterbuilder.FieldDef{Key: "factual_end", Type: filterq.FieldTypeDate, Label: pageCtx.T("ProjectStages.List.FactualEndDate"), Nullable: true},
		filterbuilder.FieldDef{Key: "created_at", Type: filterq.FieldTypeDate, Label: pageCtx.T("CreatedAt")},
	)
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
