package elxolding

import (
	"github.com/iota-agency/iota-erp/internal/modules/elxolding/mappers"
	"github.com/iota-agency/iota-erp/internal/modules/elxolding/viewmodels"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/modules/elxolding/templates/pages/login"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewLoginController(app *services.Application) shared.Controller {
	return &LoginController{
		app: app,
	}
}

type LoginController struct {
	app *services.Application
}

func (c *LoginController) Register(r *mux.Router) {
	r.HandleFunc("/login", c.Get).Methods(http.MethodGet)
	r.HandleFunc("/login", c.Post).Methods(http.MethodPost)
}

func (c *LoginController) Get(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, composables.NewPageData("Login.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	viewUsers := make([]*viewmodels.User, 0)
	users, err := c.app.UserService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, user := range users {
		viewUsers = append(viewUsers, mappers.UserToViewModel(user))
	}
	props := &login.LoginPageProps{
		PageContext: pageCtx,
		Users:       viewUsers,
	}
	if err := login.Index(props).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *LoginController) Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userId, err := strconv.ParseInt(r.FormValue("userId"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	password := r.FormValue("password")
	if userId == 0 || password == "" {
		http.Error(w, "userId or password is empty", http.StatusBadRequest)
		return
	}
	cookie, err := c.app.AuthService.CoockieAuthenticateWithUserId(r.Context(), uint(userId), password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
