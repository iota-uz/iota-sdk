//go:build dev

package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/error_pages"
	showcase "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/showcase"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	lenspostgres "github.com/iota-uz/iota-sdk/pkg/lens/postgres"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	sdkshowcase "github.com/iota-uz/iota-sdk/pkg/showcase"
)

type ShowcaseController struct {
	app      application.Application
	basePath string
	ds       lens.DataSource
}

func NewShowcaseController(app application.Application) application.Controller {
	config := configuration.Use()
	ds, err := lenspostgres.New(lenspostgres.Config{
		ConnectionString: config.Database.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
	})
	if err != nil {
		log.Printf("Failed to create lens data source for showcase: %v", err)
		return &ShowcaseController{app: app, basePath: "/_dev"}
	}
	return &ShowcaseController{app: app, basePath: "/_dev", ds: ds}
}

func (c *ShowcaseController) Key() string {
	return c.basePath
}

func (c *ShowcaseController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.ProvideLocalizer(c.app),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", di.H(c.Overview)).Methods(http.MethodGet)
	router.HandleFunc("/components/form", di.H(c.Form)).Methods(http.MethodGet)
	router.HandleFunc("/components/other", di.H(c.Other)).Methods(http.MethodGet)
	router.HandleFunc("/components/kanban", di.H(c.Kanban)).Methods(http.MethodGet)
	router.HandleFunc("/components/loaders", di.H(c.Loaders)).Methods(http.MethodGet)
	router.HandleFunc("/components/charts", di.H(c.Charts)).Methods(http.MethodGet)
	router.HandleFunc("/components/tooltips", di.H(c.Tooltips)).Methods(http.MethodGet)
	router.HandleFunc("/lens", di.H(c.Lens)).Methods(http.MethodGet)
	router.HandleFunc("/error-pages/403", di.H(c.Error403Page)).Methods(http.MethodGet)
	router.HandleFunc("/error-pages/404", di.H(c.Error404Page)).Methods(http.MethodGet)
	router.HandleFunc("/error-preview/403", di.H(c.Error403Preview)).Methods(http.MethodGet)
	router.HandleFunc("/error-preview/404", di.H(c.Error404Preview)).Methods(http.MethodGet)
	router.HandleFunc("/api/showcase/toast-example", di.H(c.ToastExample)).Methods(http.MethodPost)

	// Dynamic routes for registered categories
	for _, cat := range sdkshowcase.GlobalRegistry().GetAllCategories() {
		categoryPath := cat.Path
		// Create closure to capture categoryPath
		router.HandleFunc("/components/"+categoryPath, di.H(func(
			r *http.Request,
			w http.ResponseWriter,
			logger *logrus.Entry,
		) {
			c.CustomCategoryPage(categoryPath, r, w, logger)
		})).Methods(http.MethodGet)
	}

	log.Printf(
		"See %s%s for docs\n",
		configuration.Use().Origin,
		c.basePath,
	)
}

func (c *ShowcaseController) getSidebarProps() sidebar.Props {
	componentItems := []sidebar.Item{
		sidebar.NewLink(fmt.Sprintf("%s/components/form", c.basePath), "Form", nil),
		sidebar.NewLink(fmt.Sprintf("%s/components/loaders", c.basePath), "Loaders", nil),
		sidebar.NewLink(fmt.Sprintf("%s/components/charts", c.basePath), "Charts", nil),
		sidebar.NewLink(fmt.Sprintf("%s/components/tooltips", c.basePath), "Tooltips", nil),
		sidebar.NewLink(fmt.Sprintf("%s/components/other", c.basePath), "Other", nil),
		sidebar.NewLink(fmt.Sprintf("%s/components/kanban", c.basePath), "Kanban", nil),
	}

	// Add registered categories
	for _, cat := range sdkshowcase.GlobalRegistry().GetAllCategories() {
		componentItems = append(componentItems,
			sidebar.NewLink(
				fmt.Sprintf("%s/components/%s", c.basePath, cat.Path),
				cat.Name,
				nil,
			),
		)
	}

	items := []sidebar.Item{
		sidebar.NewLink(c.basePath, "Overview", nil),
		sidebar.NewLink(fmt.Sprintf("%s/lens", c.basePath), "Lens Dashboard", icons.MagnifyingGlass(icons.Props{Size: "20"})),
		sidebar.NewLink(fmt.Sprintf("%s/crud", c.basePath), "Crud", icons.Buildings(icons.Props{Size: "20"})),
		sidebar.NewGroup(
			"Components",
			icons.PuzzlePiece(icons.Props{Size: "20"}),
			componentItems,
		),
		sidebar.NewGroup(
			"Error Pages",
			icons.Warning(icons.Props{Size: "20"}),
			[]sidebar.Item{
				sidebar.NewLink(fmt.Sprintf("%s/error-pages/403", c.basePath), "403 Forbidden", nil),
				sidebar.NewLink(fmt.Sprintf("%s/error-pages/404", c.basePath), "404 Not Found", nil),
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
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.OverviewPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Form(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.FormPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Other(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.OtherPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Kanban(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.KanbanPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Loaders(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.LoadersPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Charts(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.ChartsPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Tooltips(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.TooltipsPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Lens(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	dash := lens.NewDashboard("IOTA SDK Core Analytics",
		lens.NewRow(
			lens.Line("user-registrations", "User Registrations Over Time").
				Query("SELECT DATE(created_at) as label, COUNT(*)::float as value FROM users WHERE created_at >= NOW() - INTERVAL '30 days' GROUP BY DATE(created_at) ORDER BY label").
				Span(6).Build(),
			lens.Bar("user-languages", "User Interface Languages").
				Query("SELECT ui_language as label, COUNT(*)::float as value FROM users GROUP BY ui_language ORDER BY value DESC").
				Span(6).Build(),
		),
		lens.NewRow(
			lens.Pie("user-types", "User Type Distribution").
				Query("SELECT type as label, COUNT(*)::float as value FROM users GROUP BY type").
				Legend().Span(4).Build(),
			lens.Gauge("session-activity", "Active Sessions").
				Query("SELECT COUNT(*)::float as value FROM sessions WHERE expires_at > NOW()").
				Colors("#f59e0b").Span(4).Build(),
			lens.Table("recent-users", "Recently Registered Users").
				Query("SELECT first_name, last_name, email, ui_language, created_at FROM users ORDER BY created_at DESC LIMIT 10").
				Span(4).Build(),
		),
	)

	var results *lens.Results
	if c.ds != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		results = lens.Execute(ctx, c.ds, dash)
	}

	props := showcase.LensPageProps{
		SidebarProps: c.getSidebarProps(),
		Dashboard:    dash,
		Results:      results,
	}
	templ.Handler(showcase.LensPage(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error403Page(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.Error403Page(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error404Page(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	props := showcase.IndexPageProps{
		SidebarProps: c.getSidebarProps(),
	}
	templ.Handler(showcase.Error404Page(props)).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error403Preview(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	w.WriteHeader(http.StatusForbidden)
	templ.Handler(error_pages.ForbiddenContent()).ServeHTTP(w, r)
}

func (c *ShowcaseController) Error404Preview(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	w.WriteHeader(http.StatusNotFound)
	templ.Handler(error_pages.NotFoundContent()).ServeHTTP(w, r)
}

func (c *ShowcaseController) ToastExample(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	htmx.ToastSuccess(w, "Server Response", "This toast was triggered by an HTMX request!")
	w.WriteHeader(http.StatusOK)
}

func (c *ShowcaseController) CustomCategoryPage(
	categoryPath string,
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	cat, ok := sdkshowcase.GlobalRegistry().GetCategory(categoryPath)
	if !ok {
		http.NotFound(w, r)
		return
	}

	props := showcase.CustomCategoryPageProps{
		SidebarProps: c.getSidebarProps(),
		Category:     cat,
	}
	templ.Handler(showcase.CustomCategoryPage(props)).ServeHTTP(w, r)
}
