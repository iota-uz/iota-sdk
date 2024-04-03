package roles

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/apollos-studio/sso/models"
	"github.com/apollos-studio/sso/pkg/server/helpers"
	"github.com/apollos-studio/sso/pkg/server/routes"
	"github.com/apollos-studio/sso/templates/pages/roles"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"net/http"
)

type Route struct {
	Db *sqlx.DB
}

func (i *Route) Prefix() string {
	return "/roles"
}

func (i *Route) Setup(router *mux.Router, opts *routes.Options) {
	i.Db = opts.Db
	router.Use(helpers.RequireAuthMiddleware)
	router.HandleFunc("/", i.Get).Methods(http.MethodGet)
	router.HandleFunc("/{id}", i.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/{id}", i.FormSubmit).Methods(http.MethodPost)
}

func (i *Route) Get(w http.ResponseWriter, r *http.Request) {
	var items []*models.Role
	err := i.Db.Select(&items, "SELECT * FROM roles")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	data := &roles.IndexData{
		Roles: items,
	}
	if err := roles.Index(data).Render(r.Context(), w); err != nil {
		helpers.ServerError(w, err)
		return
	}
}

func (i *Route) GetOne(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	role := &models.Role{}
	if id != "new" {
		err := i.Db.Get(role, "SELECT * FROM roles WHERE id = $1", id)
		if err != nil {
			helpers.ServerError(w, err)
			return
		}
	}
	var resources []*roles.ResourceHtml
	var temp string
	if id == "new" {
		temp = "0"
	} else {
		temp = id
	}
	err := i.Db.Select(&resources, `SELECT id, description, 
       (SELECT role_id FROM resources_roles WHERE resource_id = resources.id AND role_id = $1) is not null checked FROM resources;`, temp)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	data := &roles.EditData{
		Role:      role,
		Resources: resources,
	}
	if err := roles.EditPage(data).Render(r.Context(), w); err != nil {
		helpers.ServerError(w, err)
		return
	}
}

func (i *Route) handleResources(r *http.Request, roleId int64) error {
	resources := r.Form["resources[]"]
	tx, err := i.Db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	if len(resources) == 0 {
		_, err := tx.Exec("DELETE FROM resources_roles WHERE role_id = $1", roleId)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	q, args, err := sqlx.In("DELETE FROM resources_roles WHERE resource_id NOT IN (?) AND role_id=?", resources, roleId)
	if err != nil {
		return err
	}
	_, err = tx.Exec(i.Db.Rebind(q), args...)
	if err != nil {
		return err
	}
	for _, v := range resources {
		_, err := tx.Exec(
			"INSERT INTO resources_roles (resource_id, role_id) VALUES ($1, $2) ON CONFLICT (resource_id, role_id) DO NOTHING",
			v,
			roleId,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (i *Route) save(r *http.Request) error {
	id := mux.Vars(r)["id"]
	role := &models.Role{}
	if id != "new" {
		if err := i.Db.Get(role, "SELECT * FROM roles WHERE id = $1", id); err != nil {
			return err
		}
	}
	role.Name = r.FormValue("name")
	role.Description = models.JsonNullString{
		NullString: sql.NullString{
			String: r.FormValue("description"),
			Valid:  true,
		},
	}
	if errs := role.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		return errors.New(response["errors"][0].Message)
	}
	if err := role.Save(i.Db); err != nil {
		return err
	}
	if err := i.handleResources(r, role.Id); err != nil {
		return err
	}
	return nil
}

func (i *Route) delete(r *http.Request) error {
	id := mux.Vars(r)["id"]
	if _, err := i.Db.Exec("DELETE FROM roles WHERE id = $1", id); err != nil {
		return err
	}
	return nil
}

func (i *Route) FormSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	action := r.FormValue("action")

	var err error
	if action == "delete" {
		err = i.delete(r)
	} else {
		err = i.save(r)
	}
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	http.Redirect(w, r, "/roles", http.StatusFound)
}

type ApiRoute struct {
	Db *sqlx.DB
}

type PostData struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (i *ApiRoute) Prefix() string {
	return "/api/roles"
}

func (i *ApiRoute) Setup(router *mux.Router, opts *routes.Options) {
	i.Db = opts.Db
	router.HandleFunc("/", i.Find).Methods(http.MethodGet)
	router.HandleFunc("/", i.Post).Methods(http.MethodPost)
	router.HandleFunc("/{id}", i.Get).Methods(http.MethodGet)
	router.HandleFunc("/{id}", i.Remove).Methods(http.MethodDelete)
}

func (i *ApiRoute) Find(w http.ResponseWriter, r *http.Request) {
	var roles []*models.Role
	if err := i.Db.Select(&roles, "SELECT * FROM roles"); err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, roles)
}

func (i *ApiRoute) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	role := models.Role{}
	if err := i.Db.Get(&role, "SELECT * FROM roles WHERE id = $1", id); err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, role)
}

func (i *ApiRoute) Remove(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if _, err := i.Db.Exec("DELETE FROM roles WHERE id = $1", id); err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, map[string]string{"message": "success"})
}

func (i *ApiRoute) Post(w http.ResponseWriter, r *http.Request) {
	data := PostData{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	role := models.Role{
		Name: data.Name,
		Description: models.JsonNullString{
			NullString: sql.NullString{
				String: data.Description,
				Valid:  true,
			},
		},
	}
	if errs := role.Validate(); len(errs) != 0 {
		response := map[string][]*models.ValidationError{"errors": errs}
		helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
		return
	}
	if err := role.Save(i.Db); err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, role)
}
