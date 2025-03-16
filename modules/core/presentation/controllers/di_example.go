package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/scaffold"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func Handler(
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
	r.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	r.HandleFunc("/di-example/{id:[0-9]+}", scaffold.NewDIHandler(Handler).Handler())
}
