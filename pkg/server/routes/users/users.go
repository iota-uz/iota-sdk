package users

import (
	"encoding/json"
	"github.com/apollos-studio/sso/models"
	"github.com/apollos-studio/sso/pkg/server/helpers"
	"github.com/apollos-studio/sso/pkg/server/routes"
	"github.com/apollos-studio/sso/templates/pages/users"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strconv"
)

type Route struct {
	Db *sqlx.DB
}

func (u *Route) Prefix() string {
	return "/users"
}

func (u *Route) Setup(router *mux.Router, opts *routes.Options) {
	u.Db = opts.Db
	//router.Use(helpers.RequireAuthMiddleware)
	router.HandleFunc("/", u.Get).Methods(http.MethodGet)
	router.HandleFunc("/{id}", u.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/{id}", u.FormSubmit).Methods(http.MethodPost)
}

func (u *Route) Get(w http.ResponseWriter, r *http.Request) {
	var items []*models.User
	err := u.Db.Select(&items, "SELECT * FROM users")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	data := &users.IndexData{
		Users: items,
	}
	if err := users.Index(data).Render(r.Context(), w); err != nil {
		return
	}
}

func (u *Route) GetOne(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	user := &models.User{}
	if id != "new" {
		err := u.Db.Get(user, "SELECT * FROM users WHERE id = $1", id)
		if err != nil {
			helpers.ServerError(w, err)
			return
		}
	}
	var roles []*models.Role
	if err := u.Db.Select(&roles, "SELECT * FROM roles"); err != nil {
		helpers.ServerError(w, err)
		return
	}
	data := users.EditData{
		User:  user,
		Roles: roles,
	}
	if err := users.EditPage(&data).Render(r.Context(), w); err != nil {
		return
	}
}

func (u *Route) FormSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	user := &models.User{}
	if id != "new" {
		err := u.Db.Get(user, "SELECT * FROM users WHERE id = $1", id)
		if err != nil {
			helpers.ServerError(w, err)
			return
		}
	}
	user.FirstName = r.FormValue("first_name")
	user.LastName = r.FormValue("last_name")
	user.Email = r.FormValue("email")
	roleId, err := strconv.ParseInt(r.FormValue("role_id"), 10, 64)
	if err != nil {
		helpers.BadRequest(w, err)
		return
	}
	user.RoleId = roleId
	if r.FormValue("password") != "" {
		if err := user.SetPassword(r.FormValue("password")); err != nil {
			helpers.ServerError(w, err)
			return
		}
	}
	if errs := user.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
		return
	}
	if err := user.Save(u.Db); err != nil {
		helpers.ServerError(w, err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (u *Route) Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	user := &models.User{}
	user.FirstName = r.FormValue("first_name")
	user.LastName = r.FormValue("last_name")
	user.Email = r.FormValue("email")
	user.Password = r.FormValue("password")
	roleId, err := strconv.ParseInt(r.FormValue("role_id"), 10, 64)
	if err != nil {
		helpers.BadRequest(w, err)
		return
	}
	user.RoleId = roleId
	if errs := user.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
		return
	}
	if err := user.SetPassword(user.Password); err != nil {
		helpers.ServerError(w, err)
		return
	}
	if err := user.Save(u.Db); err != nil {
		helpers.ServerError(w, err)
		return
	}

	http.Redirect(w, r, "/users", http.StatusFound)
}

type ApiRoute struct {
	Db *sqlx.DB
}

func (u *ApiRoute) Prefix() string {
	return "/api/users"
}

type PostData struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	RoleId    int64  `json:"role_id"`
}

func (u *ApiRoute) Setup(router *mux.Router, opts *routes.Options) {
	u.Db = opts.Db
	router.HandleFunc("/", u.Get).Methods(http.MethodGet)
	router.HandleFunc("/", u.Post).Methods(http.MethodPost)
}

func (u *ApiRoute) Get(w http.ResponseWriter, r *http.Request) {
	var users []*models.User
	err := u.Db.Select(&users, "SELECT * FROM users")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, users)
}

func (u *ApiRoute) Post(w http.ResponseWriter, r *http.Request) {
	data := PostData{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	user := &models.User{
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		Password:  data.Password,
		RoleId:    data.RoleId,
	}
	if errs := user.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
		return
	}
	if err := user.SetPassword(user.Password); err != nil {
		helpers.ServerError(w, err)
		return
	}
	if err := user.Save(u.Db); err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, user)
}
