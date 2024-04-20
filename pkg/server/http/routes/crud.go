package routes

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/pkg/server/graphql/adapters"
	"github.com/iota-agency/iota-erp/pkg/server/helpers"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strconv"
)

type CrudRoute struct {
	Path    string
	Service adapters.Service
	Db      *sqlx.DB
}

func (u *CrudRoute) Prefix() string {
	return u.Path
}

func (u *CrudRoute) Setup(router *mux.Router, opts *Options) {
	u.Db = opts.Db
	router.HandleFunc("/", u.Find).Methods(http.MethodGet)
	router.HandleFunc("/{id}", u.Get).Methods(http.MethodGet)
	router.HandleFunc("/", u.Post).Methods(http.MethodPost)
	router.HandleFunc("/{id}", u.Patch).Methods(http.MethodPatch)
	router.HandleFunc("/{id}", u.Delete).Methods(http.MethodDelete)
}

func (u *CrudRoute) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		helpers.BadRequest(w, err)
		return
	}
	res, err := u.Service.Get(&adapters.GetQuery{
		Id: id,
	})
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, res)
}

func (u *CrudRoute) Find(w http.ResponseWriter, r *http.Request) {
	res, err := u.Service.Find(&adapters.FindQuery{})
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, res)
}

func (u *CrudRoute) Post(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	res, err := u.Service.Create(data)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusCreated, res)
}

func (u *CrudRoute) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		helpers.BadRequest(w, err)
		return
	}
	data := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		helpers.BadRequest(w, err)
		return
	}
	res, err := u.Service.Patch(id, data)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusOK, res)
}

func (u *CrudRoute) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		helpers.BadRequest(w, err)
		return
	}
	err = u.Service.Remove(id)
	if err != nil {
		helpers.ServerError(w, err)
		return
	}
	helpers.RespondWithJson(w, http.StatusNoContent, nil)
}
