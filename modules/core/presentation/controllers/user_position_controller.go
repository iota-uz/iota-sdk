// Package controllers provides this package.
package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/positions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
)

const (
	opPositionsList   serrors.Op = "core.controllers.PositionsController.List"
	opPositionsNew    serrors.Op = "core.controllers.PositionsController.GetNew"
	opPositionsEdit   serrors.Op = "core.controllers.PositionsController.GetEdit"
	opPositionsCreate serrors.Op = "core.controllers.PositionsController.Create"
	opPositionsUpdate serrors.Op = "core.controllers.PositionsController.Update"
	opPositionsDelete serrors.Op = "core.controllers.PositionsController.Delete"
)

type PositionsController struct {
	app      application.Application
	basePath string
}

func NewPositionsController(app application.Application) application.Controller {
	return &PositionsController{
		app:      app,
		basePath: "/positions",
	}
}

func (c *PositionsController) Key() string {
	return c.basePath
}

func (c *PositionsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	router.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	router.HandleFunc("/{id:[a-f0-9-]+}", di.H(c.GetEdit)).Methods(http.MethodGet)

	router.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[a-f0-9-]+}", di.H(c.Update)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[a-f0-9-]+}", di.H(c.Delete)).Methods(http.MethodDelete)
}

// userPickerData returns the user-select options plus a userID -> display-name
// lookup for the list view.
func (c *PositionsController) userPickerData(
	r *http.Request,
	userService *services.UserService,
) ([]*positions.UserOption, map[string]string, error) {
	users, err := userService.GetAll(r.Context())
	if err != nil {
		return nil, nil, err
	}
	opts := make([]*positions.UserOption, 0, len(users))
	names := make(map[string]string, len(users))
	for _, u := range users {
		vm := mappers.UserToViewModel(u)
		opts = append(opts, &positions.UserOption{ID: vm.ID, Name: vm.Title()})
		names[vm.ID] = vm.Title()
	}
	return opts, names, nil
}

// departmentPickerData returns the department-select options plus a
// departmentID -> localized-name lookup for the list view.
func (c *PositionsController) departmentPickerData(
	r *http.Request,
	departmentService *services.DepartmentService,
) ([]*positions.DepartmentOption, map[string]string, error) {
	locale := localeOf(r)
	depts, err := departmentService.GetAll(r.Context())
	if err != nil {
		return nil, nil, err
	}
	opts := make([]*positions.DepartmentOption, 0, len(depts))
	names := make(map[string]string, len(depts))
	for _, d := range depts {
		vm := mappers.DepartmentToViewModel(d, locale, nil)
		opts = append(opts, &positions.DepartmentOption{ID: vm.ID, Name: vm.Name})
		names[vm.ID] = vm.Name
	}
	return opts, names, nil
}

// userNamesForPage resolves the userID -> display-name lookup for just the
// rows on the current page, fetching only the referenced users via GetByIDs
// instead of loading every tenant user.
func (c *PositionsController) userNamesForPage(
	r *http.Request,
	userService *services.UserService,
	entities []userposition.UserPosition,
) (map[string]string, error) {
	seen := make(map[uint]struct{}, len(entities))
	ids := make([]uint, 0, len(entities))
	for _, p := range entities {
		uid := p.UserID()
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		ids = append(ids, uid)
	}
	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	users, err := userService.GetByIDs(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(users))
	for _, u := range users {
		vm := mappers.UserToViewModel(u)
		names[vm.ID] = vm.Title()
	}
	return names, nil
}

// deptNamesForPage resolves the departmentID -> localized-name lookup for just
// the rows on the current page, fetching only the referenced departments via
// GetByIDs instead of loading every tenant department.
func (c *PositionsController) deptNamesForPage(
	r *http.Request,
	departmentService *services.DepartmentService,
	entities []userposition.UserPosition,
	locale string,
) (map[string]string, error) {
	seen := make(map[uuid.UUID]struct{}, len(entities))
	ids := make([]uuid.UUID, 0, len(entities))
	for _, p := range entities {
		did := p.DepartmentID()
		if _, ok := seen[did]; ok {
			continue
		}
		seen[did] = struct{}{}
		ids = append(ids, did)
	}
	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	depts, err := departmentService.GetByIDs(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(depts))
	for _, d := range depts {
		vm := mappers.DepartmentToViewModel(d, locale, nil)
		names[vm.ID] = vm.Name
	}
	return names, nil
}

func (c *PositionsController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.UserPositionService,
	userService *services.UserService,
	departmentService *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.PositionRead); err != nil {
		RenderForbidden(w, r)
		return
	}
	params := composables.UsePaginated(r)
	search := r.URL.Query().Get("name")

	findParams := &userposition.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Search: search,
		SortBy: userposition.SortBy{
			Fields: []repo.SortByField[userposition.Field]{
				{Field: userposition.CreatedAtField, Ascending: false},
			},
		},
	}

	total, err := service.Count(r.Context(), findParams)
	if err != nil {
		logger.Error(serrors.E(opPositionsList, err))
		http.Error(w, "Error retrieving positions", http.StatusInternalServerError)
		return
	}

	entities, err := service.GetPaginated(r.Context(), findParams)
	if err != nil {
		logger.Error(serrors.E(opPositionsList, err))
		http.Error(w, "Error retrieving positions", http.StatusInternalServerError)
		return
	}

	locale := localeOf(r)

	// Resolve only the user/department names referenced by the current page's
	// rows (label lookup), instead of loading every tenant user and department.
	userNames, err := c.userNamesForPage(r, userService, entities)
	if err != nil {
		logger.Error(serrors.E(opPositionsList, err))
		http.Error(w, "Error retrieving positions", http.StatusInternalServerError)
		return
	}
	deptNames, err := c.deptNamesForPage(r, departmentService, entities, locale)
	if err != nil {
		logger.Error(serrors.E(opPositionsList, err))
		http.Error(w, "Error retrieving positions", http.StatusInternalServerError)
		return
	}

	viewModels := make([]*viewmodels.UserPosition, 0, len(entities))
	for _, p := range entities {
		viewModels = append(viewModels, mappers.UserPositionToViewModel(p, locale, userNames, deptNames))
	}

	pageProps := &positions.IndexPageProps{
		Positions: viewModels,
		Page:      params.Page,
		PerPage:   params.Limit,
		Search:    search,
		HasMore:   total > int64(params.Page*params.Limit),
	}

	if htmx.IsHxRequest(r) {
		if params.Page > 1 {
			templ.Handler(positions.PositionRows(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			templ.Handler(positions.PositionsTable(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
		}
	} else {
		templ.Handler(positions.Index(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *PositionsController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	userService *services.UserService,
	departmentService *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.PositionCreate); err != nil {
		RenderForbidden(w, r)
		return
	}
	userOpts, _, err := c.userPickerData(r, userService)
	if err != nil {
		logger.Error(serrors.E(opPositionsNew, err))
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}
	deptOpts, _, err := c.departmentPickerData(r, departmentService)
	if err != nil {
		logger.Error(serrors.E(opPositionsNew, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	props := &positions.CreateFormProps{
		Position: &positions.PositionFormData{Status: string(userposition.StatusActive)},
		UserOpts: userOpts,
		DeptOpts: deptOpts,
		Errors:   map[string]string{},
	}
	templ.Handler(positions.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.UserPositionService,
	userService *services.UserService,
	departmentService *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.PositionRead); err != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entity, err := service.GetByID(r.Context(), id)
	if err != nil {
		logger.Error(serrors.E(opPositionsEdit, err))
		http.Error(w, "Position not found", http.StatusNotFound)
		return
	}

	userOpts, userNames, err := c.userPickerData(r, userService)
	if err != nil {
		logger.Error(serrors.E(opPositionsEdit, err))
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}
	deptOpts, deptNames, err := c.departmentPickerData(r, departmentService)
	if err != nil {
		logger.Error(serrors.E(opPositionsEdit, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	props := &positions.EditFormProps{
		Position: mappers.UserPositionToViewModel(entity, localeOf(r), userNames, deptNames),
		UserOpts: userOpts,
		DeptOpts: deptOpts,
		Errors:   map[string]string{},
	}
	templ.Handler(positions.EditPositionDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.UserPositionService,
	userService *services.UserService,
	departmentService *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.PositionCreate); err != nil {
		RenderForbidden(w, r)
		return
	}
	dto, err := composables.UseForm(&dtos.CreateUserPositionDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		userOpts, _, err := c.userPickerData(r, userService)
		if err != nil {
			logger.Error(serrors.E(opPositionsCreate, err))
			http.Error(w, "Error retrieving users", http.StatusInternalServerError)
			return
		}
		deptOpts, _, err := c.departmentPickerData(r, departmentService)
		if err != nil {
			logger.Error(serrors.E(opPositionsCreate, err))
			http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
			return
		}
		props := &positions.CreateFormProps{
			Position: &positions.PositionFormData{
				Title:        dto.Title,
				UserID:       formUserID(r),
				DepartmentID: dto.DepartmentID,
				IsManager:    dto.IsManager,
				IsPrimary:    dto.IsPrimary,
				Status:       dto.Status,
			},
			UserOpts: userOpts,
			DeptOpts: deptOpts,
			Errors:   errors,
		}
		templ.Handler(positions.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The aggregate is created with a nil tenant; the service validates the
	// entity tenant against the caller before saving, so set it from context
	// first (mirrors GroupsController.Create).
	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		logger.Error(serrors.E(opPositionsCreate, err))
		http.Error(w, "Error getting tenant", http.StatusInternalServerError)
		return
	}
	entity = entity.SetTenantID(tenantID)

	if _, err := service.Create(r.Context(), entity); err != nil {
		logger.Error(serrors.E(opPositionsCreate, err))
		http.Error(w, "Error creating position", http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		if r.FormValue("form") == "drawer-form" {
			htmx.SetTrigger(w, "closeDrawer", `{"id": "new-position-drawer"}`)
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.UserPositionService,
	userService *services.UserService,
	departmentService *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.PositionUpdate); err != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&dtos.UpdateUserPositionDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		userOpts, userNames, err := c.userPickerData(r, userService)
		if err != nil {
			logger.Error(serrors.E(opPositionsUpdate, err))
			http.Error(w, "Error retrieving users", http.StatusInternalServerError)
			return
		}
		deptOpts, _, err := c.departmentPickerData(r, departmentService)
		if err != nil {
			logger.Error(serrors.E(opPositionsUpdate, err))
			http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
			return
		}
		userID := formUserID(r)
		props := &positions.EditFormProps{
			Position: &viewmodels.UserPosition{
				ID:           id.String(),
				UserID:       userID,
				UserName:     userNames[userID],
				DepartmentID: dto.DepartmentID,
				TitleI18n:    dto.Title,
				IsManager:    dto.IsManager,
				IsPrimary:    dto.IsPrimary,
				Status:       dto.Status,
				CanUpdate:    true,
				CanDelete:    true,
			},
			UserOpts: userOpts,
			DeptOpts: deptOpts,
			Errors:   errors,
		}
		templ.Handler(positions.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	existing, err := service.GetByID(r.Context(), id)
	if err != nil {
		logger.Error(serrors.E(opPositionsUpdate, err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := dto.Apply(existing)
	if err != nil {
		logger.Errorf("Error updating position: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := service.Update(r.Context(), entity); err != nil {
		logger.Error(serrors.E(opPositionsUpdate, err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		htmx.SetTrigger(w, "closeDrawer", `{"id": "edit-position-drawer"}`)
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.UserPositionService,
) {
	if err := composables.CanUser(r.Context(), permissions.PositionDelete); err != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := service.Delete(r.Context(), id); err != nil {
		logger.Error(serrors.E(opPositionsDelete, err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

// formUserID returns the submitted user id as a string for re-rendering the
// form's picker after a validation failure.
func formUserID(r *http.Request) string {
	return r.FormValue("UserID")
}
