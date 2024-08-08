package controllers

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/users"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UserController struct {
	app *services.Application
}

func NewUserController(app *services.Application) *UserController {
	return &UserController{
		app: app,
	}
}

func (c *UserController) Users(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	localizer, found := composables.UseLocalizer(r.Context())
	if !found {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}

	pageCtx := &types.PageContext{
		Localizer: localizer,
		Pathname:  pathname,
		Title:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Users"}),
	}

	params := composables.UsePaginated(r)
	us, err := c.app.UserService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		http.Error(w, "Error retreving users", http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("HX-Request")) > 0
	if isHxRequest {
		templ.Handler(users.UsersTable(localizer, us), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(users.Index(pageCtx, us), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *UserController) GetEdit(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	localizer, found := composables.UseLocalizer(r.Context())
	if !found {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}
	roles, err := c.app.RoleService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retreving roles", http.StatusInternalServerError)
		return
	}
	us, err := c.app.UserService.GetByID(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Error retreving users", http.StatusInternalServerError)
		return
	}
	pageCtx := &types.PageContext{
		Localizer: localizer,
		Pathname:  pathname,
		Title:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "EditUser"}),
	}
	templ.Handler(users.Edit(pageCtx, us, roles), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UserController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	action := r.FormValue("_action")
	var err error
	if action == "save" {
		err = c.app.UserService.Update(r.Context(), &user.User{
			Id:        int64(id),
			FirstName: r.FormValue("firstName"),
			LastName:  r.FormValue("lastName"),
			Email:     r.FormValue("email"),
		})
	} else if action == "delete" {
		_, err = c.app.UserService.Delete(r.Context(), int64(id))
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (c *UserController) GetNew(w http.ResponseWriter, r *http.Request) {
	pathname := r.URL.Path
	localizer, found := composables.UseLocalizer(r.Context())
	if !found {
		http.Error(w, "localizer not found", http.StatusInternalServerError)
		return
	}
	roles, err := c.app.RoleService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retreving roles", http.StatusInternalServerError)
		return
	}
	pageCtx := &types.PageContext{
		Localizer: localizer,
		Pathname:  pathname,
		Title:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NewUser"}),
	}
	templ.Handler(users.New(pageCtx, roles), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UserController) CreateUser(w http.ResponseWriter, r *http.Request) {
	user := &user.User{
		FirstName: r.FormValue("firstName"),
		LastName:  r.FormValue("lastName"),
		Email:     r.FormValue("email"),
	}

	user.SetPassword(r.FormValue("password"))
	if err := c.app.UserService.Create(r.Context(), user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/users", http.StatusFound)
}
