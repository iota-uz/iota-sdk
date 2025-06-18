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
	showcaseui "github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/showcase"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/lens/builder"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource/postgres"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type ShowcaseController struct {
	app      application.Application
	basePath string
	executor executor.Executor
}

func NewShowcaseController(app application.Application) application.Controller {
	// Setup PostgreSQL data source for lens
	config := configuration.Use()
	pgConfig := postgres.Config{
		ConnectionString: config.Database.ConnectionString(),
		MaxConnections:   5,
		MinConnections:   1,
		QueryTimeout:     30 * time.Second,
	}
	
	pgDataSource, err := postgres.NewPostgreSQLDataSource(pgConfig)
	if err != nil {
		log.Printf("Failed to create PostgreSQL data source for lens: %v", err)
		// Create controller without executor if data source fails
		return &ShowcaseController{
			app:      app,
			basePath: "/_dev",
			executor: nil,
		}
	}
	
	// Create executor and register data source
	exec := executor.NewExecutor(nil, 30*time.Second)
	err = exec.RegisterDataSource("core", pgDataSource)
	if err != nil {
		log.Printf("Failed to register data source: %v", err)
		pgDataSource.Close()
		exec = nil
	}

	controller := &ShowcaseController{
		app:      app,
		basePath: "/_dev",
		executor: exec,
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
	router.HandleFunc("/lens", di.H(c.Lens)).Methods(http.MethodGet)

	log.Printf(
		"See %s%s for docs\n",
		configuration.Use().Address(),
		c.basePath,
	)
}

func (c *ShowcaseController) getSidebarProps() sidebar.Props {
	items := []sidebar.Item{
		sidebar.NewLink(c.basePath, "Overview", nil),
		sidebar.NewLink(fmt.Sprintf("%s/lens", c.basePath), "Lens Dashboard", icons.MagnifyingGlass(icons.Props{Size: "20"})),
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

func (c *ShowcaseController) Lens(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	// Create dashboard configuration
	dashboard := builder.NewDashboard().
		ID("showcase-dashboard").
		Title("IOTA SDK Core Analytics").
		Description("Core system analytics dashboard using real database tables").
		Grid(12, 120).
		Variable("timeRange", "30d").
		Variable("tenant", "current").
		Panel(
			builder.LineChart().
				ID("user-registrations").
				Title("User Registrations Over Time").
				Position(0, 0).
				Size(6, 4).
				DataSource("core").
				Query("SELECT DATE(created_at) as timestamp, COUNT(*)::float as value FROM users WHERE created_at >= NOW() - INTERVAL '30 days' GROUP BY DATE(created_at) ORDER BY timestamp").
				Option("yAxis", map[string]interface{}{
					"label": "New Users",
				}).
				Option("xAxis", map[string]interface{}{
					"type": "datetime",
					"label": "Date",
				}).
				Build(),
		).
		Panel(
			builder.BarChart().
				ID("user-languages").
				Title("User Interface Languages").
				Position(6, 0).
				Size(6, 4).
				DataSource("core").
				Query("SELECT ui_language as timestamp, COUNT(*)::float as value FROM users GROUP BY ui_language ORDER BY value DESC").
				Option("yAxis", map[string]interface{}{
					"label": "User Count",
				}).
				Build(),
		).
		Panel(
			builder.PieChart().
				ID("user-types").
				Title("User Type Distribution").
				Position(0, 4).
				Size(4, 4).
				DataSource("core").
				Query("SELECT type as timestamp, COUNT(*)::float as value FROM users GROUP BY type").
				Option("showLegend", true).
				Option("showLabels", true).
				Build(),
		).
		Panel(
			builder.GaugeChart().
				ID("session-activity").
				Title("Active Sessions").
				Position(4, 4).
				Size(4, 4).
				DataSource("core").
				Query("SELECT NOW() as timestamp, COUNT(*)::float as value FROM sessions WHERE expires_at > NOW()").
				Option("min", 0).
				Option("max", 1000).
				Option("unit", "sessions").
				Build(),
		).
		Panel(
			builder.TableChart().
				ID("recent-users").
				Title("Recently Registered Users").
				Position(8, 4).
				Size(4, 8).
				DataSource("core").
				Query("SELECT first_name, last_name, email, ui_language, created_at FROM users ORDER BY created_at DESC LIMIT 10").
				Option("pageSize", 5).
				Option("sortable", true).
				Option("columns", []map[string]interface{}{
					{"field": "first_name", "header": "First Name", "type": "text"},
					{"field": "last_name", "header": "Last Name", "type": "text"},
					{"field": "email", "header": "Email", "type": "text"},
					{"field": "ui_language", "header": "Language", "type": "badge"},
					{"field": "created_at", "header": "Registered", "type": "datetime"},
				}).
				Build(),
		).
		Build()

	// Execute dashboard queries if executor is available
	var dashboardResult *executor.DashboardResult
	if c.executor != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		
		result, err := c.executor.ExecuteDashboard(ctx, dashboard)
		if err != nil {
			logger.WithError(err).Error("Failed to execute dashboard queries")
			// Continue with empty result
			dashboardResult = &executor.DashboardResult{
				PanelResults: make(map[string]*executor.ExecutionResult),
				Duration:     0,
				Errors:       []error{err},
				ExecutedAt:   time.Now(),
			}
		} else {
			dashboardResult = result
		}
	}

	props := showcaseui.LensPageProps{
		SidebarProps:    c.getSidebarProps(),
		Dashboard:       dashboard,
		DashboardResult: dashboardResult,
	}
	templ.Handler(showcaseui.LensPage(props)).ServeHTTP(w, r)
}
