package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	showcaseui "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/showcase"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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
	router.HandleFunc("", di.H(c.Overview)).Methods(http.MethodGet)
	router.HandleFunc("/components/form", di.H(c.Form)).Methods(http.MethodGet)
	router.HandleFunc("/components/other", di.H(c.Other)).Methods(http.MethodGet)
	router.HandleFunc("/components/loaders", di.H(c.Loaders)).Methods(http.MethodGet)
	router.HandleFunc("/components/charts", di.H(c.Charts)).Methods(http.MethodGet)

	log.Printf(
		"See %s%s for docs\n",
		configuration.Use().Address(),
		c.basePath,
	)
}

func (c *ShowcaseController) getSidebarProps() sidebar.Props {
	items := []sidebar.Item{
		sidebar.NewLink(c.basePath, "Overview", nil),
		sidebar.NewGroup(
			"Components",
			icons.PuzzlePiece(icons.Props{Size: "20"}),
			[]sidebar.Item{
				sidebar.NewLink(fmt.Sprintf("%s/components/form", c.basePath), "Form", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/loaders", c.basePath), "Loaders", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/charts", c.basePath), "Charts", nil),
				sidebar.NewLink(fmt.Sprintf("%s/components/other", c.basePath), "Other", nil),
			},
		),
	}

	tabGroups := sidebar.TabGroupCollection{
		Groups: []sidebar.TabGroup{
			{
				Label: "Showcase",
				Value: "showcase",
				Items: items,
			},
		},
		DefaultValue: "showcase",
	}

	return sidebar.Props{
		TabGroups: tabGroups,
	}
}

func (c *ShowcaseController) Overview(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcaseui.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcaseui.OverviewPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Form(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcaseui.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcaseui.FormPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Other(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcaseui.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcaseui.OtherPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Loaders(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcaseui.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcaseui.LoadersPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Charts(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcaseui.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcaseui.ChartsPage(props)).ServeHTTP(w, r)
}
