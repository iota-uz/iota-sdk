package controllers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/pages/login"
	"github.com/iota-agency/iota-sdk/pkg/service"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/types"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/composables"
)

func SetFlash(w http.ResponseWriter, name string, value []byte) {
	c := &http.Cookie{Name: name, Value: base64.URLEncoding.EncodeToString(value)}
	http.SetCookie(w, c)
}

func SetFlashMap[K comparable, V any](w http.ResponseWriter, name string, value map[K]V) {
	errors, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SetFlash(w, name, errors)
}

type LoginDTO struct {
	Email    string `validate:"required"`
	Password string `validate:"required"`
}

func (e *LoginDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(e)
	if errs == nil {
		return errorMessages, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		errorMessages[err.Field()] = err.Translate(l)
	}
	return errorMessages, len(errorMessages) == 0
}

func NewLoginController(app application.Application) application.Controller {
	return &LoginController{
		app:         app,
		authService: app.Service(services.AuthService{}).(*services.AuthService),
	}
}

type LoginController struct {
	app         application.Application
	authService *services.AuthService
}

func (c *LoginController) Register(r *mux.Router) {
	r.HandleFunc("/oauth/google/callback", c.authService.OauthGoogleCallback)
	r.HandleFunc("/login", c.Get).Methods(http.MethodGet)
	r.HandleFunc("/login", c.Post).Methods(http.MethodPost)
}

func (c *LoginController) Get(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	errorsMap, err := composables.UseFlashMap[string, string](w, r, "errorsMap")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	errorMessage, err := composables.UseFlash(w, r, "error")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Login.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := login.Index(&login.LoginProps{
		PageContext:  pageCtx,
		ErrorsMap:    errorsMap,
		Email:        email,
		ErrorMessage: string(errorMessage),
	}).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *LoginController) Post(w http.ResponseWriter, r *http.Request) {
	dto := LoginDTO{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
	} //nolint:exhaustruct

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("Login.Meta.Title", ""))
	if err != nil {
		SetFlash(w, "error", []byte(pageCtx.T("Errors.Internal")))
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		SetFlashMap(w, "errorsMap", errorsMap)
		http.Redirect(w, r, "/login?email="+dto.Email, http.StatusFound)
		return
	}

	cookie, err := c.authService.CookieAuthenticate(r.Context(), dto.Email, dto.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidPassword) {
			SetFlash(w, "error", []byte(pageCtx.T("Login.Errors.PasswordInvalid")))
		} else {
			SetFlash(w, "error", []byte(pageCtx.T("Errors.Internal")))
		}
		http.Redirect(w, r, "/login?email="+dto.Email, http.StatusFound)
		return
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
