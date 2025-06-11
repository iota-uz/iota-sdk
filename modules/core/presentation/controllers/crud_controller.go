package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/crud_v2"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"net/http"
)

type CrudController[TEntity any] struct {
	basePath string
	app      application.Application
	builder  crud_v2.Builder[TEntity]
}

func NewCrudController[TEntity any](
	basePath string,
	app application.Application,
	builder crud_v2.Builder[TEntity],
) application.Controller {
	return &CrudController[TEntity]{
		basePath: basePath,
		app:      app,
		builder:  builder,
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

func (c *CrudController[TEntity]) List(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) GetNew(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) GetEdit(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Create(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Update(w http.ResponseWriter, r *http.Request) {}

func (c *CrudController[TEntity]) Delete(w http.ResponseWriter, r *http.Request) {}
