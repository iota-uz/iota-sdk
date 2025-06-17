package controllers

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/crud_pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"log"
	"net/http"
)

type CrudController[TEntity any] struct {
	basePath string
	app      application.Application
	schema   crud.Schema[TEntity]
	service  crud.Service[TEntity]
}

func NewCrudController[TEntity any](
	basePath string,
	app application.Application,
	builder crud.Builder[TEntity],
) application.Controller {
	return &CrudController[TEntity]{
		basePath: basePath,
		app:      app,
		schema:   builder.Schema(),
		service:  builder.Service(),
	}
}

func (c *CrudController[TEntity]) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)

	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("/{id}", c.GetEdit).Methods(http.MethodGet)

	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id}", c.Delete).Methods(http.MethodDelete)
}

func (c *CrudController[TEntity]) Key() string {
	return c.basePath
}

func (c *CrudController[TEntity]) List(w http.ResponseWriter, r *http.Request) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&crud.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
	}, r)

	// Get the list of entities
	entities, err := c.service.List(r.Context(), params)
	if err != nil {
		log.Printf("Failed to get entities: %v", err)
		http.Error(w, "Error retrieving entities", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := c.service.Count(r.Context(), params)
	if err != nil {
		log.Printf("Failed to count entities: %v", err)
		http.Error(w, "Error counting entities", http.StatusInternalServerError)
		return
	}

	// Convert entities to field values for display
	var rows [][]crud.FieldValue
	for _, entity := range entities {
		fieldValues, err := c.schema.Mapper().ToFieldValues(r.Context(), entity)
		if err != nil {
			log.Printf("Failed to map entity to field values: %v", err)
			continue
		}
		rows = append(rows, fieldValues)
	}

	// Prepare data for the template
	props := &crud_pages.IndexPageProps[TEntity]{
		BasePath:        c.basePath,
		Schema:          c.schema,
		Rows:            rows,
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
	}

	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	if isHxRequest {
		templ.Handler(crud_pages.ListTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(crud_pages.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *CrudController[TEntity]) GetNew(w http.ResponseWriter, r *http.Request) {
	// Create empty entity for the form
	var entity TEntity

	// Convert to field values
	fieldValues, err := c.schema.Mapper().ToFieldValues(r.Context(), entity)
	if err != nil {
		log.Printf("Failed to map entity to field values: %v", err)
		http.Error(w, "Error preparing form", http.StatusInternalServerError)
		return
	}

	props := &crud_pages.CreatePageProps[TEntity]{
		Schema:   c.schema,
		Fields:   fieldValues,
		Errors:   map[string]string{},
		BasePath: c.basePath,
	}

	templ.Handler(crud_pages.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *CrudController[TEntity]) GetEdit(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create entity from form data
	var entity TEntity
	fieldValues, err := c.schema.Mapper().ToFieldValues(r.Context(), entity)
	if err != nil {
		log.Printf("Failed to map entity to field values: %v", err)
		http.Error(w, "Error processing form", http.StatusInternalServerError)
		return
	}

	// Update field values from form
	for i, fv := range fieldValues {
		field := fv.Field()
		if !field.Hidden() && !field.Key() && !field.Readonly() {
			formValue := r.FormValue(field.Name())
			if formValue != "" {
				// Update the field value with form data
				fieldValues[i] = field.Value(formValue)
			}
		}
	}

	// Convert back to entity
	entity, err = c.schema.Mapper().ToEntity(r.Context(), fieldValues)
	if err != nil {
		log.Printf("Failed to map field values to entity: %v", err)
		http.Error(w, "Error processing form", http.StatusInternalServerError)
		return
	}

	// Save entity
	savedEntity, err := c.service.Save(r.Context(), entity)
	if err != nil {
		log.Printf("Failed to save entity: %v", err)

		// Return form with errors
		props := &crud_pages.CreatePageProps[TEntity]{
			Schema:   c.schema,
			Fields:   fieldValues,
			Errors:   map[string]string{"_error": err.Error()},
			BasePath: c.basePath,
		}

		templ.Handler(crud_pages.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Get the primary key value for redirect
	savedFieldValues, err := c.schema.Mapper().ToFieldValues(r.Context(), savedEntity)
	if err != nil {
		log.Printf("Failed to map saved entity: %v", err)
		http.Error(w, "Error processing saved entity", http.StatusInternalServerError)
		return
	}

	var primaryKeyValue string
	for _, fv := range savedFieldValues {
		if fv.Field().Key() {
			primaryKeyValue = fmt.Sprintf("%v", fv.Value())
			break
		}
	}

	// Redirect to list or edit page
	if htmx.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", fmt.Sprintf("%s/%s", c.basePath, primaryKeyValue))
	} else {
		http.Redirect(w, r, c.basePath, http.StatusSeeOther)
	}
}

func (c *CrudController[TEntity]) Update(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Delete(w http.ResponseWriter, r *http.Request) {}
