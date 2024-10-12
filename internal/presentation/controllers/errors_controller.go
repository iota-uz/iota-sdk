package controllers

import (
	"github.com/a-h/templ"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/error_pages"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
)

type ErrorController struct {
}

func NewErrorController(app *services.Application) Controller {
	return &ErrorController{}
}

func (c *ErrorController) Register(r *mux.Router) {
	r.NotFoundHandler = http.HandlerFunc(c.NotFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(c.MethodNotAllowed)
}

func (c *ErrorController) NotFound(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, composables.NewPageData("ErrorPages.NotFound.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &error_pages.NotFoundPageProps{
		PageContext: pageCtx,
	}
	templ.Handler(error_pages.NotFoundContent(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *ErrorController) MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}
