package controllers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/tab"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/presentation/mappers"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/pages/account"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/shared/middleware"
	"github.com/iota-agency/iota-sdk/pkg/types"

	"github.com/gorilla/mux"
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

func (c *AccountController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.Get).Methods(http.MethodGet)
	router.HandleFunc("/settings", c.GetSettings).Methods(http.MethodGet)
	router.HandleFunc("/settings", c.PostSettings).Methods(http.MethodPost)
	router.HandleFunc("", c.Post).Methods(http.MethodPost)
}

func (c *AccountController) defaultProps(r *http.Request, errors map[string]string) (*account.ProfilePageProps, error) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Account.Meta.Index.Title", ""),
	)
	if err != nil {
		return nil, err
	}
	nonNilErrors := make(map[string]string)
	if errors != nil {
		nonNilErrors = errors
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		return nil, err
	}
	props := &account.ProfilePageProps{
		PageContext: pageCtx,
		PostPath:    c.basePath,
		User:        mappers.UserToViewModel(u),
		Errors:      nonNilErrors,
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

func (c *AccountController) Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto := SaveAccountDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uniTranslator, ok := composables.UseUniLocalizer(r.Context())
	if !ok {
		http.Error(w, composables.ErrLocalizerNotFound.Error(), http.StatusInternalServerError)
	}
	errors, ok := dto.Ok(uniTranslator)
	if !ok {
		props, err := c.defaultProps(r, errors)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		templ.Handler(account.ProfileForm(props)).ServeHTTP(w, r)
		return
	}
	u, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entity, err := dto.ToEntity(u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := c.userService.Update(r.Context(), entity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props, err := c.defaultProps(r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(account.ProfileForm(&account.ProfilePageProps{
		PageContext: props.PageContext,
		User:        mappers.UserToViewModel(entity),
		Errors:      map[string]string{},
	})).ServeHTTP(w, r)
}

func (c *AccountController) GetSettings(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("Account.Meta.Settings.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tabs, err := composables.UseTabs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allNavItems, err := composables.UseAllNavItems(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tabViewModels := mapping.MapViewModels(tabs, mappers.TabToViewModel)
	props := &account.SettingsPageProps{
		PageContext: pageCtx,
		AllNavItems: allNavItems,
		Tabs:        tabViewModels,
	}
	templ.Handler(account.Settings(props)).ServeHTTP(w, r)
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
	dtos := make([]*tab.CreateDTO, 0, len(hDto.Hrefs))
	for i, href := range hDto.Hrefs {
		dtos = append(dtos, &tab.CreateDTO{
			Href:     href,
			Position: uint(i),
			UserID:   u.ID,
		})
	}
	if err := c.tabService.Delete(r.Context(), &tab.DeleteParams{UserID: u.ID}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := c.tabService.CreateMany(r.Context(), dtos); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/account/settings", http.StatusFound)
}
