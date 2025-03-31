package controllers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"

	"github.com/gorilla/mux"
	scaffoldui "github.com/iota-uz/iota-sdk/components/scaffold"
	scaffoldfilters "github.com/iota-uz/iota-sdk/components/scaffold/filters"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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
	subRouter := r.PathPrefix("/di-example").Subrouter()
	subRouter.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	subRouter.HandleFunc("/{id:[0-9]+}", di.NewHandler(c.Handler).Handler())
	subRouter.HandleFunc("/scaffold-table", di.NewHandler(c.ScaffoldTable).Handler())
}

func (c *DIEmployeeController) Handler(
	// these will be auto injected by di handler
	r *http.Request,
	w http.ResponseWriter,
	localizer *i18n.Localizer,
	u user.User,
	currencyService *services.CurrencyService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	message := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "NavigationLinks.Dashboard",
		},
	})

	currencies, err := currencyService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("NavigationLinks.Dashboard: %s", message)))
	w.Write([]byte("\n"))
	w.Write([]byte(fmt.Sprintf("Fullname: %s %s", u.FirstName(), u.LastName())))
	w.Write([]byte("\n"))
	w.Write([]byte(fmt.Sprintf("ID: %d", id)))
	w.Write([]byte("\n"))

	for _, c := range currencies {
		w.Write([]byte(fmt.Sprintf("Currency: %s", c.Name)))
		w.Write([]byte("\n"))
	}
}

func (c *DIEmployeeController) ScaffoldTable(
	r *http.Request, w http.ResponseWriter,
	userService *services.UserService,
	roleService *services.RoleService,
) {

	users, err := userService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles, err := roleService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	createdAtFilter := scaffoldfilters.NewFilter(
		"CreatedAt",
		scaffoldfilters.WithPlaceholder("Created at"),
	)
	createdAtFilter.AddOpt("today", "Today").
		AddOpt("yesterday", "Yesterday").
		AddOpt("last7days", "Last 7 days").
		AddOpt("last30days", "Last 30 days")
	roleFilter := scaffoldfilters.NewFilter(
		"RoleID",
		scaffoldfilters.WithPlaceholder("Role"),
		scaffoldfilters.MultiSelect(),
	)

	for _, r := range roles {
		roleFilter.AddOpt(fmt.Sprintf("%d", r.ID()), r.Name())
	}

	tableConfig := scaffoldui.NewTableConfig("Users", "/di-example/scaffold-table")
	tableConfig.AddFilters(
		createdAtFilter,
		roleFilter,
	)

	tableConfig.AddColumn("fullname", "Fullname")
	tableConfig.AddColumn("email", "Email")
	tableConfig.AddColumn("phone", "Phone")
	tableConfig.AddColumn("createdAt", "Created At", scaffoldui.WithDateFormatter())

	tableData := scaffoldui.NewData()

	for _, c := range users {
		tableData.AddItem(map[string]interface{}{
			"fullname":  c.FirstName() + " " + c.LastName(),
			"email":     c.Email(),
			"phone":     c.Phone(),
			"createdAt": c.CreatedAt(),
		})
	}

	if htmx.IsHxRequest(r) {
		templ.Handler(scaffoldui.Table(tableConfig, tableData)).ServeHTTP(w, r)
	} else {
		templ.Handler(scaffoldui.Page(tableConfig, tableData)).ServeHTTP(w, r)
	}
}