package controllers

import (
	"net/http"
	"strconv"

	"github.com/iota-agency/iota-erp/internal/presentation/types"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/entities/role"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/users"
	"github.com/iota-agency/iota-erp/pkg/composables"
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
	r.HandleFunc(c.basePath, c.Users).Methods(http.MethodGet)
	r.HandleFunc(c.basePath, c.CreateUser).Methods(http.MethodPost)
	r.HandleFunc(c.basePath+"/new", c.GetNew).Methods(http.MethodGet)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.DeleteUser).Methods(http.MethodDelete)
}

func (c *UsersController) Users(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Users.Meta.List.Title"}) //nolint:exhaustruct
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
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Users.Meta.Edit.Title"}) //nolint:exhaustruct
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	roles, err := c.app.RoleService.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
		return
	}

	us, err := c.app.UserService.GetByID(r.Context(), int64(id))
	if err != nil {
		http.Error(w, "Error retrieving users", http.StatusInternalServerError)
		return
	}
	templ.Handler(users.Edit(pageCtx, us, roles, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.app.UserService.Delete(r.Context(), int64(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *UsersController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := FormAction(r.FormValue("_action"))
	if !action.IsValid() {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	switch action {
	case FormActionDelete:
		_, err = c.app.UserService.Delete(r.Context(), int64(id))
	case FormActionSave:
		var roleID int
		roleID, err = strconv.Atoi(r.FormValue("roleID"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		upd := &user.UpdateDTO{ //nolint:exhaustruct
			FirstName: r.FormValue("firstName"),
			LastName:  r.FormValue("lastName"),
			Email:     r.FormValue("email"),
			RoleID:    int64(roleID),
		}
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, &composables.PageData{Title: "Users.Meta.Edit.Title"}) //nolint:exhaustruct
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if errors, ok := upd.Ok(pageCtx.UniTranslator); !ok {
			roles, err := c.app.RoleService.GetAll(r.Context())
			if err != nil {
				http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
				return
			}

			us, err := c.app.UserService.GetByID(r.Context(), int64(id))
			if err != nil {
				http.Error(w, "Error retrieving users", http.StatusInternalServerError)
				return
			}

			templ.Handler(users.EditForm(pageCtx.Localizer, us, roles, errors), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.UserService.Update(r.Context(), &user.User{ //nolint:exhaustruct
			ID:        int64(id),
			FirstName: upd.FirstName,
			LastName:  upd.LastName,
			Email:     upd.Email,
			Roles:     []*role.Role{{ID: upd.RoleID}},
		})
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
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Users.Meta.New.Title"}) //nolint:exhaustruct
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(users.New(pageCtx, roles, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *UsersController) CreateUser(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("password")
	roleID, err := strconv.Atoi(r.FormValue("roleID"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userEntity := user.User{ //nolint:exhaustruct
		FirstName: r.FormValue("firstName"),
		LastName:  r.FormValue("lastName"),
		Email:     r.FormValue("email"),
		Password:  &password,
		Roles: []*role.Role{
			{ID: int64(roleID)},
		},
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Users.Meta.New.Title"}) //nolint:exhaustruct
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := userEntity.Ok(pageCtx.UniTranslator); !ok {
		roles, err := c.app.RoleService.GetAll(r.Context())
		if err != nil {
			http.Error(w, "Error retrieving roles", http.StatusInternalServerError)
			return
		}
		templ.Handler(users.CreateForm(pageCtx.Localizer, userEntity, roles, errors), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := userEntity.SetPassword(password); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := c.app.UserService.Create(r.Context(), &userEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, c.basePath)
}
