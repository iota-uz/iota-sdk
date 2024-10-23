package controllers

import (
	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/users"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/middleware"
	"net/http"
)

type UsersController struct {
	app      *services.Application
	basePath string
}

func NewUsersController(app *services.Application) Controller {
	return &UsersController{
		app:      app,
		basePath: "/users",
	}
}

func (c *UsersController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.Users).Methods(http.MethodGet)
	router.HandleFunc("", c.CreateUser).Methods(http.MethodPost)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.DeleteUser).Methods(http.MethodDelete)
}

func (c *UsersController) Users(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r, composables.NewPageData("Users.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := composables.UsePaginated(r)
	us, err := c.app.UserService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	if isHxRequest {
		templ.Handler(users.UsersTable(pageCtx.Localizer, us), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(users.Index(pageCtx, us), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *UsersController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r, composables.NewPageData("Users.Meta.Edit.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles, err := c.app.RoleService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	us, err := c.app.UserService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}
	props := &users.EditFormProps{
		PageContext: pageCtx,
		User:        us,
		Roles:       roles,
		Errors:      map[string]string{},
	}
	templ.Handler(users.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := c.app.UserService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *UsersController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	action := FormAction(r.FormValue("_action"))
	if !action.IsValid() {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	switch action {
	case FormActionDelete:
		_, err = c.app.UserService.Delete(r.Context(), id)
	case FormActionSave:
		dto := &user.UpdateDTO{} //nolint:exhaustruct
		if err = decoder.Decode(dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var pageCtx *composables.PageContext
		pageCtx, err = composables.UsePageCtx(
			r, composables.NewPageData("Users.Meta.Edit.Title", ""),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if errors, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			roles, err := c.app.RoleService.GetAll(r.Context())
			if err != nil {
				http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
				return
			}

			us, err := c.app.UserService.GetByID(r.Context(), id)
			if err != nil {
				http.Error(w, "Error retrieving users", http.StatusInternalServerError)
				return
			}

			props := &users.EditFormProps{
				PageContext: pageCtx,
				User:        us,
				Roles:       roles,
				Errors:      errors,
			}
			templ.Handler(users.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.UserService.Update(r.Context(), dto.ToEntity(id))
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *UsersController) GetNew(w http.ResponseWriter, r *http.Request) {
	roles, err := c.app.RoleService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}
	pageCtx, err := composables.UsePageCtx(r, composables.NewPageData("Users.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &users.CreateFormProps{
		PageContext: pageCtx,
		User:        user.User{}, //nolint:exhaustruct
		Roles:       roles,
		Errors:      map[string]string{},
	}
	templ.Handler(users.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) CreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := &user.CreateDTO{} //nolint:exhaustruct
	if err := decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, composables.NewPageData("Users.Meta.New.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		roles, err := c.app.RoleService.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}
		props := &users.CreateFormProps{
			PageContext: pageCtx,
			User:        *dto.ToEntity(),
			Roles:       roles,
			Errors:      errors,
		}
		templ.Handler(
			users.CreateForm(props), templ.WithStreaming(),
		).ServeHTTP(w, r)
		return
	}

	if err := c.app.UserService.Create(r.Context(), dto.ToEntity()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, c.basePath)
}
