package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/filters"
	sfilters "github.com/iota-uz/iota-sdk/components/scaffold/filters"
	fbuilder "github.com/iota-uz/iota-sdk/components/scaffold/form"
	tbuilder "github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

//    code varchar(3) NOT NULL PRIMARY KEY, -- RUB
//    name varchar(255) NOT NULL, -- Russian Ruble
//    symbol varchar(3) NOT NULL, -- ₽
//    created_at timestamp with time zone DEFAULT now(),
//    updated_at timestamp with time zone DEFAULT now()

type Currency struct {
	Code    string
	Name    string
	Symbol  string
	Created time.Time
	Updated time.Time
}

func NewDIExampleController(app application.Application) application.Controller {
	return &DIEmployeeController{
		app:      app,
		basePath: "/di",
	}
}

type DIEmployeeController struct {
	app      application.Application
	basePath string
}

func (c *DIEmployeeController) Key() string {
	return c.basePath
}

func (c *DIEmployeeController) Register(r *mux.Router) {
	subRouter := r.PathPrefix(c.basePath).Subrouter()
	subRouter.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	subRouter.HandleFunc("/sf-table", di.H(c.ScaffoldTable))
	subRouter.HandleFunc("/sf-table/{id:[0-9]+}", di.H(c.Details))
	subRouter.HandleFunc("/sf-table/new", di.H(c.New))
}

func (c *DIEmployeeController) ScaffoldTable(
	r *http.Request, w http.ResponseWriter,
	userService *services.UserService,
	roleService *services.RoleService,
	logger *logrus.Entry,
) {
	params := &user.FindParams{}

	if v := r.URL.Query().Get("Search"); v != "" {
		params.Search = v
	}

	if r.URL.Query().Get("RoleID") != "" {
		params.Filters = append(params.Filters, user.Filter{
			Column: user.RoleID,
			Filter: repo.In(r.URL.Query()["RoleID"]),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.From"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.WithError(err).Error("failed to parse CreatedAt.From")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params.Filters = append(params.Filters, user.Filter{
			Column: user.CreatedAt,
			Filter: repo.Gte(t),
		})
	}

	if v := r.URL.Query().Get("CreatedAt.To"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			logger.WithError(err).Error("failed to parse CreatedAt.To")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params.Filters = append(params.Filters, user.Filter{
			Column: user.CreatedAt,
			Filter: repo.Lte(t),
		})
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
	roleFilter := sfilters.NewFilter(
		"RoleID",
		sfilters.WithPlaceholder("Role"),
		sfilters.MultiSelect(),
	)

	for _, r := range roles {
		roleFilter.Add(sfilters.Opt(fmt.Sprintf("%d", r.ID()), r.Name()))
	}

	tcfg := tbuilder.NewTableConfig(
		"Users",
		fmt.Sprintf("%s/sf-table", c.basePath),
	).AddFilters(filters.CreatedAt())
	tcfg.AddCols(
		tbuilder.Column("fullname", "Fullname"),
		tbuilder.Column("email", "Email"),
		tbuilder.Column("createdAt", "Created At"),
	)
	tcfg.SetSideFilter(roleFilter.AsSideFilter())
	for _, u := range users {
		fetchUrl := fmt.Sprintf("/%s/sf-table/%d", c.basePath, u.ID())
		tcfg.AddRows(
			tbuilder.Row(
				templ.Raw(u.FirstName()+" "+u.LastName()),
				templ.Raw(u.Email().Value()),
				tbuilder.DateTime(u.CreatedAt()),
			).ApplyOpts(tbuilder.WithDrawer(fetchUrl)),
		)
	}

	if htmx.IsHxRequest(r) {
		err = tbuilder.Rows(tcfg).Render(r.Context(), w)
	} else {
		err = tbuilder.Page(tcfg).Render(r.Context(), w)
	}

	if err != nil {
		logger.WithError(err).Error("failed to render table")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *DIEmployeeController) Details(
	r *http.Request, w http.ResponseWriter,
) {
	props := tbuilder.DefaultDrawerProps{
		Title:       "User Details",
		CallbackURL: fmt.Sprintf("%s/sf-table", c.basePath),
	}
	templ.Handler(tbuilder.DefaultDrawer(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *DIEmployeeController) New(
	r *http.Request, w http.ResponseWriter,
	logger *logrus.Entry,
) {
	cfg := fbuilder.NewFormConfig(
		"New Currency",
		fmt.Sprintf("%s/sf-table", c.basePath),
		fmt.Sprintf("%s/sf-table", c.basePath),
		"Create",
	).Add(
		fbuilder.Text("code", "Code").Required().Default("USD").Build(),
		fbuilder.Text("name", "Name").Required().Build(),
		fbuilder.Text("symbol", "Symbol").Required().Build(),
		fbuilder.Color("color", "Color").Required().Build(),
	)

	templ.Handler(fbuilder.Page(cfg), templ.WithStreaming()).ServeHTTP(w, r)
}
