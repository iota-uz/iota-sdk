package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	sfui "github.com/iota-uz/iota-sdk/components/scaffold"
	"github.com/iota-uz/iota-sdk/components/scaffold/filters"
	scaffoldfilters "github.com/iota-uz/iota-sdk/components/scaffold/filters"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

func NewDIExampleController(app application.Application) application.Controller {
	return &DIEmployeeController{
		app: app,
	}
}

type DIEmployeeController struct {
	app application.Application
}

func (c *DIEmployeeController) Key() string {
	return "/di-example"
}

func (c *DIEmployeeController) Register(r *mux.Router) {
	subRouter := r.PathPrefix("/di").Subrouter()
	subRouter.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	subRouter.HandleFunc("/scaffold-table", di.H(c.ScaffoldTable))
}

func (c *DIEmployeeController) ScaffoldTable(
	r *http.Request, w http.ResponseWriter,
	userService *services.UserService,
	roleService *services.RoleService,
	logger *logrus.Entry,
) {
	params := &user.FindParams{
		Search: r.URL.Query().Get("search"),
	}

	users, err := userService.GetPaginated(r.Context(), params)
	if err != nil {
		logger.WithError(err).Error("failed to get users")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get roles")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	createdAtFilter := scaffoldfilters.NewFilter(
		"CreatedAt",
		scaffoldfilters.WithPlaceholder("Created at"),
	)
	createdAtFilter.Add(
		filters.Opt("today", "Today"),
		filters.Opt("thisWeek", "This Week"),
		filters.Opt("thisMonth", "This Month"),
		filters.Opt("thisYear", "This Year"),
		filters.Opt("lastYear", "Last Year"),
	)
	roleFilter := scaffoldfilters.NewFilter(
		"RoleID",
		scaffoldfilters.WithPlaceholder("Role"),
		scaffoldfilters.MultiSelect(),
	)

	for _, r := range roles {
		roleFilter.Add(filters.Opt(fmt.Sprintf("%d", r.ID()), r.Name()))
	}

	tcfg := sfui.NewTableConfig("Users", "/di/scaffold-table")
	tcfg.AddFilters(
		roleFilter,
		createdAtFilter,
	)

	tcfg.AddCols(
		sfui.Column("fullname", "Fullname"),
		sfui.Column("email", "Email"),
		sfui.Column("createdAt", "Created At"),
	)
	for _, c := range users {
		tcfg.AddRows(
			sfui.Row(
				templ.Raw(c.FirstName()+" "+c.LastName()),
				templ.Raw(c.Email().Value()),
				sfui.DateTime(c.CreatedAt()),
			),
		)
	}

	if htmx.IsHxRequest(r) {
		err = sfui.Rows(tcfg).Render(r.Context(), w)
	} else {
		err = sfui.Page(tcfg).Render(r.Context(), w)
	}

	if err != nil {
		logger.WithError(err).Error("failed to render table")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
