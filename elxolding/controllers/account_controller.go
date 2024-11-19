package controllers

import (
	"github.com/a-h/templ"
	"github.com/iota-agency/iota-sdk/elxolding/mappers"
	"github.com/iota-agency/iota-sdk/elxolding/templates/pages/account"
	"github.com/iota-agency/iota-sdk/internal/application"
	"github.com/iota-agency/iota-sdk/internal/modules/shared"
	"github.com/iota-agency/iota-sdk/internal/modules/shared/middleware"
	"github.com/iota-agency/iota-sdk/internal/services"
	"github.com/iota-agency/iota-sdk/internal/types"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"net/http"

	"github.com/gorilla/mux"
)

type AccountController struct {
	app         *application.Application
	userService *services.UserService
	basePath    string
}

func NewAccountController(app *application.Application) shared.Controller {
	return &AccountController{
		app:         app,
		userService: app.Service(services.UserService{}).(*services.UserService),
		basePath:    "/account",
	}
}

func (c *AccountController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.Get).Methods(http.MethodGet)
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
		UserData:    mappers.UserToViewModel(u),
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
		UserData:    mappers.UserToViewModel(entity),
		Errors:      map[string]string{},
	})).ServeHTTP(w, r)
}
