package controllers

import (
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/account"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type AccountController struct {
	app         application.Application
	userService *services.UserService
	tabService  *services.TabService
	basePath    string
}

func NewAccountController(app application.Application) application.Controller {
	return &AccountController{
		app:         app,
		userService: app.Service(services.UserService{}).(*services.UserService),
		tabService:  app.Service(services.TabService{}).(*services.TabService),
		basePath:    "/account",
	}
}

func (c *AccountController) Key() string {
	return c.basePath
}

func (c *AccountController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.Get).Methods(http.MethodGet)
	getRouter.HandleFunc("/sidebar", c.GetSettings).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.HandleFunc("", c.Update).Methods(http.MethodPost)
	setRouter.HandleFunc("/sidebar", c.PostSettings).Methods(http.MethodPost)
}

func (c *AccountController) defaultProps(r *http.Request, errors map[string]string) (*account.ProfilePageProps, error) {
	nonNilErrors := make(map[string]string)
	if errors != nil {
		nonNilErrors = errors
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		return nil, err
	}
	props := &account.ProfilePageProps{
		PostPath: c.basePath,
		User:     mappers.UserToViewModel(u),
		Errors:   nonNilErrors,
	}
	return props, nil
}

func (c *AccountController) Get(w http.ResponseWriter, r *http.Request) {
	props, err := c.defaultProps(r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(account.Index(props)).ServeHTTP(w, r)
}

func (c *AccountController) Update(w http.ResponseWriter, r *http.Request) {
	logger := composables.UseLogger(r.Context())
	dto, err := composables.UseForm(&dtos.SaveAccountDTO{}, r)
	if err != nil {
		logger.WithError(err).Error("failed to parse form")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors, ok := dto.Ok(r.Context()); !ok {
		props, err := c.defaultProps(r, errors)
		if err != nil {
			logger.WithError(err).Error("failed to get default props")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		logger.WithField("errors", errors).Debug("validation failed")
		templ.Handler(account.ProfileForm(props)).ServeHTTP(w, r)
		return
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		logger.WithError(err).Error("failed to get user from context")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entity, err := dto.Apply(u)
	if err != nil {
		logger.WithError(err).Error("failed to apply dto")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := c.userService.Update(r.Context(), entity); err != nil {
		logger.WithError(err).Error("failed to update user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templ.Handler(account.ProfileForm(&account.ProfilePageProps{
		User:   mappers.UserToViewModel(entity),
		Errors: map[string]string{},
	})).ServeHTTP(w, r)
}

func (c *AccountController) GetSettings(w http.ResponseWriter, r *http.Request) {
	tabs, err := composables.UseTabs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allNavItems, err := composables.UseAllNavItems(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tabViewModels := mapping.MapViewModels(tabs, mappers.TabToViewModel)
	props := &account.SettingsPageProps{
		AllNavItems: allNavItems,
		Tabs:        tabViewModels,
	}
	templ.Handler(account.SidebarSettings(props)).ServeHTTP(w, r)
}

func (c *AccountController) PostSettings(w http.ResponseWriter, r *http.Request) {
	type hrefsDto struct {
		Hrefs []string
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hDto := hrefsDto{}
	if err := shared.Decoder.Decode(&hDto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dtos := make([]*tab.Tab, 0, len(hDto.Hrefs))
	for i, href := range hDto.Hrefs {
		dtos = append(dtos, &tab.Tab{
			Href:     href,
			Position: uint(i),
			UserID:   u.ID(),
			TenantID: u.TenantID(),
		})
	}
	if err := c.tabService.CreateManyUserTabs(r.Context(), u.ID(), dtos); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/account/sidebar", http.StatusFound)
}
