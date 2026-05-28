// Package controllers provides this package.
package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/departments"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
)

const (
	opDepartmentsList   serrors.Op = "core.controllers.DepartmentsController.List"
	opDepartmentsNew    serrors.Op = "core.controllers.DepartmentsController.GetNew"
	opDepartmentsEdit   serrors.Op = "core.controllers.DepartmentsController.GetEdit"
	opDepartmentsCreate serrors.Op = "core.controllers.DepartmentsController.Create"
	opDepartmentsUpdate serrors.Op = "core.controllers.DepartmentsController.Update"
	opDepartmentsDelete serrors.Op = "core.controllers.DepartmentsController.Delete"
)

type DepartmentsController struct {
	app      application.Application
	basePath string
}

func NewDepartmentsController(app application.Application) application.Controller {
	return &DepartmentsController{
		app:      app,
		basePath: "/departments",
	}
}

func (c *DepartmentsController) Key() string {
	return c.basePath
}

func (c *DepartmentsController) Register(r *mux.Router) {
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

// localeOf returns the request locale code (e.g. "en", "uz-Cyrl") used to
// resolve MultiLang display values.
func localeOf(r *http.Request) string {
	return composables.UsePageCtx(r.Context()).GetLocale().String()
}

// departmentNameIndex resolves a parent-name lookup (id -> localized name) for
// the whole tenant. It also returns the slice of all departments so callers can
// reuse it to build parent-select options.
func (c *DepartmentsController) departmentNameIndex(
	r *http.Request,
	service *services.DepartmentService,
) ([]*viewmodels.Department, map[string]string, error) {
	locale := localeOf(r)
	all, err := service.GetAll(r.Context())
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]string, len(all))
	vms := make([]*viewmodels.Department, 0, len(all))
	for _, d := range all {
		vm := mappers.DepartmentToViewModel(d, locale, nil)
		names[vm.ID] = vm.Name
		vms = append(vms, vm)
	}
	return vms, names, nil
}

// parentNamesForPage resolves the parentID -> localized name lookup for just
// the rows on the current page, fetching only the referenced parents via
// GetByIDs instead of loading the whole tenant.
func (c *DepartmentsController) parentNamesForPage(
	r *http.Request,
	service *services.DepartmentService,
	entities []department.Department,
	locale string,
) (map[string]string, error) {
	seen := make(map[uuid.UUID]struct{}, len(entities))
	parentIDs := make([]uuid.UUID, 0, len(entities))
	for _, d := range entities {
		pid := d.ParentID()
		if pid == nil {
			continue
		}
		if _, ok := seen[*pid]; ok {
			continue
		}
		seen[*pid] = struct{}{}
		parentIDs = append(parentIDs, *pid)
	}
	if len(parentIDs) == 0 {
		return map[string]string{}, nil
	}

	parents, err := service.GetByIDs(r.Context(), parentIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(parents))
	for _, p := range parents {
		vm := mappers.DepartmentToViewModel(p, locale, nil)
		names[vm.ID] = vm.Name
	}
	return names, nil
}

func (c *DepartmentsController) List(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.DepartmentRead); err != nil {
		RenderForbidden(w, r)
		return
	}
	params := composables.UsePaginated(r)
	search := r.URL.Query().Get("name")

	findParams := &department.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		Search: search,
		SortBy: department.SortBy{
			Fields: []repo.SortByField[department.Field]{
				{Field: department.CreatedAtField, Ascending: false},
			},
		},
	}

	total, err := service.Count(r.Context(), findParams)
	if err != nil {
		logger.Error(serrors.E(opDepartmentsList, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	entities, err := service.GetPaginated(r.Context(), findParams)
	if err != nil {
		logger.Error(serrors.E(opDepartmentsList, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	locale := localeOf(r)

	// Resolve only the parent names referenced by the current page's rows
	// (label lookup), instead of loading every tenant department.
	names, err := c.parentNamesForPage(r, service, entities, locale)
	if err != nil {
		logger.Error(serrors.E(opDepartmentsList, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	viewModels := make([]*viewmodels.Department, 0, len(entities))
	for _, d := range entities {
		viewModels = append(viewModels, mappers.DepartmentToViewModel(d, locale, names))
	}

	pageProps := &departments.IndexPageProps{
		Departments: viewModels,
		Page:        params.Page,
		PerPage:     params.Limit,
		Search:      search,
		HasMore:     total > int64(params.Page*params.Limit),
	}

	if htmx.IsHxRequest(r) {
		if params.Page > 1 {
			templ.Handler(departments.DepartmentRows(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			templ.Handler(departments.DepartmentsTable(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
		}
	} else {
		templ.Handler(departments.Index(pageProps), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *DepartmentsController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.DepartmentCreate); err != nil {
		RenderForbidden(w, r)
		return
	}
	options, err := c.parentOptions(r, service, "")
	if err != nil {
		logger.Error(serrors.E(opDepartmentsNew, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	props := &departments.CreateFormProps{
		Department: &departments.DepartmentFormData{Status: string(department.StatusActive)},
		ParentOpts: options,
		Errors:     map[string]string{},
	}
	templ.Handler(departments.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *DepartmentsController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.DepartmentService,
	orgQuery *services.OrgQueryService,
) {
	if err := composables.CanUser(r.Context(), permissions.DepartmentRead); err != nil {
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
		logger.Error(serrors.E(opDepartmentsEdit, err))
		http.Error(w, "Department not found", http.StatusNotFound)
		return
	}

	options, err := c.parentOptionsExcludingSubtree(r, service, orgQuery, id)
	if err != nil {
		logger.Error(serrors.E(opDepartmentsEdit, err))
		http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
		return
	}

	props := &departments.EditFormProps{
		Department: mappers.DepartmentToViewModel(entity, localeOf(r), nil),
		ParentOpts: options,
		Errors:     map[string]string{},
	}
	templ.Handler(departments.EditDepartmentDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *DepartmentsController) Create(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.DepartmentCreate); err != nil {
		RenderForbidden(w, r)
		return
	}
	dto, err := composables.UseForm(&dtos.CreateDepartmentDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		options, err := c.parentOptions(r, service, "")
		if err != nil {
			logger.Error(serrors.E(opDepartmentsCreate, err))
			http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
			return
		}
		props := &departments.CreateFormProps{
			Department: &departments.DepartmentFormData{
				Name:     dto.Name,
				Code:     dto.Code,
				ParentID: dto.ParentID,
				Order:    strconv.Itoa(dto.Order),
				Status:   dto.Status,
			},
			ParentOpts: options,
			Errors:     errors,
		}
		templ.Handler(departments.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		logger.Error(serrors.E(opDepartmentsCreate, err))
		http.Error(w, "Error getting tenant", http.StatusInternalServerError)
		return
	}
	entity = entity.SetTenantID(tenantID)

	if _, err := service.Create(r.Context(), entity); err != nil {
		// Surface known validation/conflict failures back to the drawer as
		// field-level errors (mirrors the DTO-error branch above).
		if status, fields := departmentValidationFieldError(r.Context(), err); fields != nil {
			options, optErr := c.parentOptions(r, service, "")
			if optErr != nil {
				logger.Error(serrors.E(opDepartmentsCreate, optErr))
				http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
				return
			}
			props := &departments.CreateFormProps{
				Department: &departments.DepartmentFormData{
					Name:     dto.Name,
					Code:     dto.Code,
					ParentID: dto.ParentID,
					Order:    strconv.Itoa(dto.Order),
					Status:   dto.Status,
				},
				ParentOpts: options,
				Errors:     fields,
			}
			w.WriteHeader(status)
			templ.Handler(departments.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		logger.Error(serrors.E(opDepartmentsCreate, err))
		http.Error(w, "Error creating department", http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		if r.FormValue("form") == "drawer-form" {
			htmx.SetTrigger(w, "closeDrawer", `{"id": "new-department-drawer"}`)
		}
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *DepartmentsController) Update(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.DepartmentService,
	orgQuery *services.OrgQueryService,
) {
	if err := composables.CanUser(r.Context(), permissions.DepartmentUpdate); err != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto, err := composables.UseForm(&dtos.UpdateDepartmentDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errors, ok := dto.Ok(r.Context()); !ok {
		options, err := c.parentOptionsExcludingSubtree(r, service, orgQuery, id)
		if err != nil {
			logger.Error(serrors.E(opDepartmentsUpdate, err))
			http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
			return
		}
		props := &departments.EditFormProps{
			Department: &viewmodels.Department{
				ID:        id.String(),
				Code:      dto.Code,
				NameI18n:  dto.Name,
				ParentID:  dto.ParentID,
				Order:     strconv.Itoa(dto.Order),
				Status:    dto.Status,
				CanUpdate: true,
				CanDelete: true,
			},
			ParentOpts: options,
			Errors:     errors,
		}
		templ.Handler(departments.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	existing, err := service.GetByID(r.Context(), id)
	if err != nil {
		logger.Error(serrors.E(opDepartmentsUpdate, err))
		http.Error(w, "Error retrieving department", http.StatusInternalServerError)
		return
	}

	entity, err := dto.Apply(existing)
	if err != nil {
		logger.Errorf("Error updating department: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := service.Update(r.Context(), entity); err != nil {
		// Surface known validation/conflict failures back to the drawer as
		// field-level errors (mirrors the DTO-error branch above).
		if status, fields := departmentValidationFieldError(r.Context(), err); fields != nil {
			options, optErr := c.parentOptionsExcludingSubtree(r, service, orgQuery, id)
			if optErr != nil {
				logger.Error(serrors.E(opDepartmentsUpdate, optErr))
				http.Error(w, "Error retrieving departments", http.StatusInternalServerError)
				return
			}
			props := &departments.EditFormProps{
				Department: &viewmodels.Department{
					ID:        id.String(),
					Code:      dto.Code,
					NameI18n:  dto.Name,
					ParentID:  dto.ParentID,
					Order:     strconv.Itoa(dto.Order),
					Status:    dto.Status,
					CanUpdate: true,
					CanDelete: true,
				},
				ParentOpts: options,
				Errors:     fields,
			}
			w.WriteHeader(status)
			templ.Handler(departments.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		logger.Error(serrors.E(opDepartmentsUpdate, err))
		http.Error(w, "Error updating department", http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		htmx.SetTrigger(w, "closeDrawer", `{"id": "edit-department-drawer"}`)
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *DepartmentsController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	service *services.DepartmentService,
) {
	if err := composables.CanUser(r.Context(), permissions.DepartmentDelete); err != nil {
		RenderForbidden(w, r)
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := service.Delete(r.Context(), id); err != nil {
		logger.Error(serrors.E(opDepartmentsDelete, err))
		http.Error(w, "Error deleting department", http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

// parentOptions returns every tenant department as a parent-select option,
// optionally excluding a single id.
func (c *DepartmentsController) parentOptions(
	r *http.Request,
	service *services.DepartmentService,
	excludeID string,
) ([]*departments.DepartmentOption, error) {
	vms, _, err := c.departmentNameIndex(r, service)
	if err != nil {
		return nil, err
	}
	return buildParentOptions(vms, map[string]struct{}{excludeID: {}}), nil
}

// parentOptionsExcludingSubtree returns parent-select options with the edited
// department and all of its descendants removed, preventing the user from
// creating a hierarchy cycle.
func (c *DepartmentsController) parentOptionsExcludingSubtree(
	r *http.Request,
	service *services.DepartmentService,
	orgQuery *services.OrgQueryService,
	id uuid.UUID,
) ([]*departments.DepartmentOption, error) {
	vms, _, err := c.departmentNameIndex(r, service)
	if err != nil {
		return nil, err
	}
	subtree, err := orgQuery.DepartmentSubtree(r.Context(), id)
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]struct{}, len(subtree)+1)
	excluded[id.String()] = struct{}{}
	for _, sid := range subtree {
		excluded[sid.String()] = struct{}{}
	}
	return buildParentOptions(vms, excluded), nil
}

func buildParentOptions(
	vms []*viewmodels.Department,
	excluded map[string]struct{},
) []*departments.DepartmentOption {
	opts := make([]*departments.DepartmentOption, 0, len(vms))
	for _, vm := range vms {
		if _, skip := excluded[vm.ID]; skip {
			continue
		}
		opts = append(opts, &departments.DepartmentOption{ID: vm.ID, Name: vm.Name})
	}
	return opts
}

// departmentValidationFieldError maps a service or repository error to the
// drawer's field-error map plus the HTTP status to send back. It returns
// (0, nil) when the error is not a known validation failure — callers fall
// through to the generic 5xx path. Sentinels live with the layer that raises
// them: services.* for write-path validation, persistence.* for storage
// constraints. Malformed UUIDs in `ParentID` are caught earlier by the DTO
// validator (`omitempty,uuid` tag) and surface through the existing DTO error
// branch, so they are not handled here.
func departmentValidationFieldError(ctx context.Context, err error) (int, map[string]string) {
	if err == nil {
		return 0, nil
	}
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		return 0, nil
	}
	tr := func(key string) string {
		return l.MustLocalize(&i18n.LocalizeConfig{MessageID: key})
	}

	switch {
	case errors.Is(err, services.ErrDepartmentSelfLoop):
		return http.StatusBadRequest, map[string]string{"ParentID": tr("Departments.Errors.SelfLoop")}
	case errors.Is(err, services.ErrDepartmentCycle):
		return http.StatusBadRequest, map[string]string{"ParentID": tr("Departments.Errors.Cycle")}
	case errors.Is(err, services.ErrDepartmentParentNotFound):
		return http.StatusBadRequest, map[string]string{"ParentID": tr("Departments.Errors.ParentNotFound")}
	case errors.Is(err, persistence.ErrDepartmentDuplicateCode):
		return http.StatusConflict, map[string]string{"Code": tr("Departments.Errors.DuplicateCode")}
	}

	// Validation errors without a known sentinel fall through to the generic
	// 5xx path. Add a sentinel for the failure first, then map it here — the
	// drawer template only renders per-field errors, so a generic "_form" entry
	// would silently disappear and look like a no-op success.

	return 0, nil
}
