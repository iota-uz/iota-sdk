package controllers

import (
	"fmt"
	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/employees"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type EmployeeController struct {
	app             application.Application
	employeeService *services.EmployeeService
	basePath        string
}

func NewEmployeeController(app application.Application) application.Controller {
	return &EmployeeController{
		app:             app,
		employeeService: app.Service(services.EmployeeService{}).(*services.EmployeeService),
		basePath:        "/operations/employees",
	}
}

func (c *EmployeeController) Key() string {
	return c.basePath
}

func (c *EmployeeController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *EmployeeController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Employees.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := composables.UsePaginated(r)
	employeeEntities, err := c.employeeService.GetPaginated(r.Context(), &employee.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: []string{"id"},
	})
	if err != nil {
		http.Error(w, errors.Wrap(err, "Error retrieving employees").Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &employees.IndexPageProps{
		PageContext: pageCtx,
		Employees:   mapping.MapViewModels(employeeEntities, mappers.EmployeeToViewModel),
		NewURL:      fmt.Sprintf("%s/new", c.basePath),
	}
	if isHxRequest {
		templ.Handler(employees.EmployeesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(employees.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *EmployeeController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Employees.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entity, err := employee.New(
		"",
		"",
		"",
		"",
		nil,
		0,
		nil,
		nil,
		nil,
		time.Now(),
		nil,
		0, "",
	)
	props := &employees.CreatePageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Employee:    mappers.EmployeeToViewModel(entity),
		PostPath:    c.basePath,
	}
	templ.Handler(employees.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *EmployeeController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Employees.Meta.Edit.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.employeeService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving account", http.StatusInternalServerError)
		return
	}
	props := &employees.EditPageProps{
		PageContext: pageCtx,
		Employee:    mappers.EmployeeToViewModel(entity),
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(employees.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *EmployeeController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&employee.CreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Employees.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := dto.ToEntity()
		if err != nil {
			// TODO: proper error handling
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &employees.CreatePageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
			Employee:    mappers.EmployeeToViewModel(entity),
			PostPath:    c.basePath,
		}
		templ.Handler(employees.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.employeeService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *EmployeeController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto, err := composables.UseForm(&employee.UpdateDTO{}, r)
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Employees.Meta.Edit.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	errorsMap, ok := dto.Ok(r.Context())
	if ok {
		if err := c.employeeService.Update(r.Context(), id, dto); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		entity, err := c.employeeService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving account", http.StatusInternalServerError)
			return
		}
		props := &employees.EditPageProps{
			PageContext: pageCtx,
			Employee:    mappers.EmployeeToViewModel(entity),
			Errors:      errorsMap,
			SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
			DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
		}
		templ.Handler(employees.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *EmployeeController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	if _, err := c.employeeService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}
