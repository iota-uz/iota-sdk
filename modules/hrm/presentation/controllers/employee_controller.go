package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/templates/pages/employees"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/shared"
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
		basePath:        "/hrm/employees",
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
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
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
		Employees: mapping.MapViewModels(employeeEntities, mappers.EmployeeToViewModel),
		NewURL:    fmt.Sprintf("%s/new", c.basePath),
	}
	if isHxRequest {
		templ.Handler(employees.EmployeesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(employees.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *EmployeeController) GetNew(w http.ResponseWriter, r *http.Request) {
	entity := employee.New(
		"",
		"",
		"",
		"",
		nil,
		money.NewFromFloat(0, string(currency.UsdCode)),
		tax.NilTin,
		tax.NilPin,
		employee.NewLanguage("", ""),
		time.Now(),
	)
	props := &employees.CreatePageProps{
		Errors:   map[string]string{},
		Employee: mappers.EmployeeToViewModel(entity),
		PostPath: c.basePath,
	}
	templ.Handler(employees.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *EmployeeController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	entity, err := c.employeeService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving account", http.StatusInternalServerError)
		return
	}
	props := &employees.EditPageProps{
		Employee:  mappers.EmployeeToViewModel(entity),
		Errors:    map[string]string{},
		SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(employees.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *EmployeeController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&employee.CreateDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		props := &employees.CreatePageProps{
			Errors: errorsMap,
			Employee: &viewmodels.Employee{
				FirstName:       dto.FirstName,
				LastName:        dto.LastName,
				Email:           dto.Email,
				Phone:           dto.Phone,
				Salary:          strconv.FormatFloat(dto.Salary, 'f', 2, 64),
				BirthDate:       time.Time(dto.BirthDate).Format(time.DateOnly),
				HireDate:        time.Time(dto.HireDate).Format(time.DateOnly),
				ResignationDate: time.Time(dto.ResignationDate).Format(time.DateOnly),
				Tin:             dto.Tin,
				Pin:             dto.Pin,
				Notes:           dto.Notes,
			},
			PostPath: c.basePath,
		}
		templ.Handler(employees.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	err = c.employeeService.Create(r.Context(), dto)
	if err != nil {
		l, ok := intl.UseLocalizer(r.Context())
		if !ok {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Check if it's a domain validation error (TIN/PIN)
		validationErrors := make(serrors.ValidationErrors)
		if errors.Is(err, tax.ErrInvalidTin) {
			validationErrors["Tin"] = serrors.NewInvalidTINError(
				"Tin",
				"Employees.Private.TIN.Label",
				err.Error(),
			)
		} else if errors.Is(err, tax.ErrInvalidPin) {
			validationErrors["Pin"] = serrors.NewInvalidPINError(
				"Pin",
				"Employees.Private.Pin.Label",
				err.Error(),
			)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Localize validation errors
		errorsMap := serrors.LocalizeValidationErrors(validationErrors, l)

		// Re-render form with errors
		props := &employees.CreatePageProps{
			Errors: errorsMap,
			Employee: &viewmodels.Employee{
				FirstName:       dto.FirstName,
				LastName:        dto.LastName,
				Email:           dto.Email,
				Phone:           dto.Phone,
				Salary:          strconv.FormatFloat(dto.Salary, 'f', 2, 64),
				BirthDate:       time.Time(dto.BirthDate).Format(time.DateOnly),
				HireDate:        time.Time(dto.HireDate).Format(time.DateOnly),
				ResignationDate: time.Time(dto.ResignationDate).Format(time.DateOnly),
				Tin:             dto.Tin,
				Pin:             dto.Pin,
				Notes:           dto.Notes,
			},
			PostPath: c.basePath,
		}
		templ.Handler(employees.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *EmployeeController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("%+v", err), http.StatusBadRequest)
		return
	}
	dto, err := composables.UseForm(&employee.UpdateDTO{}, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("%+v", err), http.StatusBadRequest)
		return
	}
	errorsMap, ok := dto.Ok(r.Context())
	if ok {
		err := c.employeeService.Update(r.Context(), id, dto)
		if err != nil {
			l, ok := intl.UseLocalizer(r.Context())
			if !ok {
				http.Error(w, fmt.Sprintf("%+v", err), http.StatusInternalServerError)
				return
			}

			// Check if it's a domain validation error (TIN/PIN)
			validationErrors := make(serrors.ValidationErrors)
			if errors.Is(err, tax.ErrInvalidTin) {
				validationErrors["Tin"] = serrors.NewInvalidTINError(
					"Tin",
					"Employees.Private.TIN.Label",
					err.Error(),
				)
			} else if errors.Is(err, tax.ErrInvalidPin) {
				validationErrors["Pin"] = serrors.NewInvalidPINError(
					"Pin",
					"Employees.Private.Pin.Label",
					err.Error(),
				)
			} else {
				http.Error(w, fmt.Sprintf("%+v", err), http.StatusInternalServerError)
				return
			}

			// Localize validation errors
			errorsMap = serrors.LocalizeValidationErrors(validationErrors, l)

			// Re-render form with errors
			entity, err := c.employeeService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving employee", http.StatusInternalServerError)
				return
			}
			props := &employees.EditPageProps{
				Employee:  mappers.EmployeeToViewModel(entity),
				Errors:    errorsMap,
				SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
				DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
			}
			templ.Handler(employees.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
	} else {
		entity, err := c.employeeService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving employee", http.StatusInternalServerError)
			return
		}
		props := &employees.EditPageProps{
			Employee:  mappers.EmployeeToViewModel(entity),
			Errors:    errorsMap,
			SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
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
