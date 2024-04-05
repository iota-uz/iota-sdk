package roles

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/pkg/server/helpers"
	"github.com/iota-agency/iota-erp/pkg/server/routes"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strconv"
)

type ApiRoute struct {
	Db *sqlx.DB
}

func (u *ApiRoute) Prefix() string {
	return "/api/roles"
}

type PostData struct {
	Name string `json:"name"`
}

func (u *ApiRoute) Setup(router *mux.Router, opts *routes.Options) {
	u.Db = opts.Db
	router.HandleFunc("/", u.Get).Methods(http.MethodGet)
	router.HandleFunc("/", u.Post).Methods(http.MethodPost)
	router.HandleFunc("/{id}", u.Patch).Methods(http.MethodPatch)
}

func (u *ApiRoute) Get(w http.ResponseWriter, r *http.Request) {
	var roles []*models.Role
	err := u.Db.Select(&roles, "SELECT * FROM roles")
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, roles)
}

func (u *ApiRoute) Post(w http.ResponseWriter, r *http.Request) {
	data := PostData{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	role := &models.Role{
		Name: data.Name,
	}
	//if errs := role.Validate(); len(errs) != 0 {
	//	response := map[string][]*models.ValidationError{"errors": errs}
	//	helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
	//}
	_, err := u.Db.NamedExec("INSERT INTO roles (name) VALUES (:name)", role)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusCreated, role)
}

func (u *ApiRoute) Patch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		helpers.BadRequest(w, err)
		return
	}
	data := PostData{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	role := &models.Role{
		Id:   id,
		Name: data.Name,
	}
	//if errs := role.Validate(); len(errs) != 0 {
	//	response := map[string][]*models.ValidationError{"errors": errs}
	//	helpers.RespondWithJson(w, http.StatusUnprocessableEntity, response)
	//}
	_, err = u.Db.NamedExec("UPDATE roles SET name=:name WHERE id=:id", role)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, role)
}
