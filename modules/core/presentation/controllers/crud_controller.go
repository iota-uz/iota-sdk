package controllers

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/crud_pages"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/repo"
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
	pagParams := composables.UsePaginated(r)

	params := crud.FindParams{
		Limit:  pagParams.Limit,
		Offset: pagParams.Offset,
	}

	// Handle search across all searchable fields
	searchQuery := r.URL.Query().Get("search")
	if searchQuery != "" {
		params.Search = searchQuery
	}

	// Handle sorting
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")
	if sortBy != "" {
		ascending := sortOrder != "desc"
		params.SortBy = crud.SortBy{
			Fields: []repo.SortByField[string]{
				{
					Field:     sortBy,
					Ascending: ascending,
					NullsLast: true,
				},
			},
		}
	}

	// Get the list of entities
	entities, err := c.service.List(r.Context(), &params)
	if err != nil {
		log.Printf("Failed to get entities: %v", err)
		http.Error(w, "Error retrieving entities", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := c.service.Count(r.Context(), &params)
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
	props := &crud_pages.ListPageProps[TEntity]{
		Schema:          c.schema,
		Rows:            rows,
		Page:            pagParams.Page,
		PerPage:         pagParams.Limit,
		Total:           total,
		HasMore:         total > int64(pagParams.Page*pagParams.Limit),
		BasePath:        c.basePath,
		Search:          searchQuery,
		SortBy:          sortBy,
		SortOrder:       sortOrder,
		PaginationState: pagination.New(c.basePath, pagParams.Page, int(total), pagParams.Limit),
	}

	// Handle HTMX requests
	if htmx.IsHxRequest(r) {
		if pagParams.Page > 1 {
			// Infinite scroll - return only new rows
			templ.Handler(crud_pages.EntityRows(props), templ.WithStreaming()).ServeHTTP(w, r)
		} else {
			// Regular HTMX request (search, sort, filter) - return only the table
			templ.Handler(crud_pages.EntitiesTable(props), templ.WithStreaming()).ServeHTTP(w, r)
		}
	} else {
		// Full page request
		templ.Handler(crud_pages.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *CrudController[TEntity]) GetNew(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) GetEdit(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Create(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Update(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Delete(w http.ResponseWriter, r *http.Request) {}
