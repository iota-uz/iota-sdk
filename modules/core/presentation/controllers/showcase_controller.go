package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	showcaseui "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/showcase"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type ShowcaseController struct {
	app      application.Application
	basePath string
}

func NewShowcaseController(app application.Application) application.Controller {
	controller := &ShowcaseController{
		app:      app,
		basePath: "/_dev",
	}

	return controller
}

func (c *ShowcaseController) Key() string {
	return c.basePath
}

func (c *ShowcaseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("/showcase", di.H(c.Showcase)).Methods(http.MethodGet)
}

func (c *ShowcaseController) Showcase(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	basePath := fmt.Sprintf("%s/showcase", c.basePath)
	props := showcaseui.IndexPageProps{
		SidebarProps: sidebar.Props{
			Items: []sidebar.Item{
				sidebar.NewLink(fmt.Sprintf("%s/input", basePath), "Inputs", nil),
				sidebar.NewLink(fmt.Sprintf("%s/filters", basePath), "Filters", nil),
			},
		},
	}
	templ.Handler(showcaseui.Index(props)).ServeHTTP(w, r)

}
